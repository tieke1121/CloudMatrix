# T17 — 高性能 Windows 宿主与远程 Host Agent 技术设计

版本：0.1；状态：G3 前置设计。

---

## 1. 业务目标与架构拓扑

在规模化机房部署中，控制端桌面（CloudMatrix UI）通常运行在管理员的工作机或主控机上，而数十上百台模拟器实例则分布在多台配备独立 GPU 的高性能 Windows Server 宿主机（Host Node）上。

### 拓扑架构

```text
[CloudMatrix 桌面主控端]
       │
       │ (版本化 TLS / RPC 长连接)
       ├─────────────────────────────────┐
       ▼                                 ▼
[Host Node 1: cloudmatrix-host]   [Host Node 2: cloudmatrix-host]
  ├─ Provider / MuMu 实例          ├─ Provider / 实例
  ├─ 本地 ADB 服务                 ├─ 本地 ADB 服务
  └─ 独立视频捕获与流服务           └─ 独立视频捕获与流服务
```

---

## 2. 版本化 RPC 契约与安全机制

### 2.1 协议帧与消息结构 (`internal/transport/rpc.go`)

所有控制报文均采用带长度前缀的 JSON 帧，格式如下：
1. **Protocol Version**：`v1.0`（版本不匹配时显式握手拒绝）；
2. **Host ID**：宿主机唯一标识（UUID 或硬件签名）；
3. **Request ID**：用于请求/响应关联的单调序列号；
4. **Auth Token**：预共享安全密钥（或 TLS 客户端证书指纹）；
5. **Action**：`heartbeat` / `start_instance` / `stop_instance` / `exec_task` / `query_task` / `request_stream`；
6. **Payload**：强类型结构体序列化。

### 2.2 幂等性保障 (Idempotent Execution)

- 针对副作用操作（如创建、克隆、启动、安装 APK 等），必须携带客户端生成的全局唯一 `task_id`；
- Host Agent 在本地维护已完成任务的有限哈希环记录（默认保留最近 1000 条）；
- 当网络抖动发生重连时，主控端重新发送相同的 `task_id`，Host Agent 直接返回既有执行状态与退出码，严禁重复启动或重复安装。

### 2.3 异常恢复与断线重连

- 心跳周期：5 秒；超时判定：15 秒（3 次心跳丢失）；
- 客户端断线后自动以指数退避重连（1s, 2s, 4s, 最大 15s）；
- 重连成功后自动执行状态对账（Reconcile），拉取远程宿主当前各实例的实际运行状态。

---

## 3. 部署边界与已知限制

1. **会话隔离限制**：Windows Server 在纯无交互服务（Session 0）下无法启动依赖 GPU 3D 加速渲染的模拟器；Host Agent 需以特定已登录交互式服务会话或桌面辅助守护进程（Tray Helper）方式运行；
2. **RDP 锁屏风险**：RDP 远程桌面会话断开可能导致 Direct3D 交换链失效，需配置虚拟显示适配器驱动（Virtual Display Driver）或保持会话不锁屏；
3. **真实服务器验证要求**：生产环境必须经过 168 小时长稳实验。在单机开发机环境下，远程功能仅交付进程级 RPC 通信集成与幂等逻辑测试，物理双机与机房长稳在无物理环境时依规标记为 `BLOCKED`。
