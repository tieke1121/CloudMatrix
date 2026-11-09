# T00 — 模拟器与宿主适配能力证据报告

- 运行标识：`run-20261107-165945-t00`
- 记录时间：2026-11-08（UTC 2026-11-07T16:59:45Z）
- 依据标准：`AGENTS.md`、`docs/01-product-and-architecture.md`、`docs/02-agent-implementation-plan.md` (T00)

---

## 1. 宿主只读环境基线

| 维度 | 实测配置 | 状态 / 说明 |
|---|---|---|
| 操作系统 | Microsoft Windows 11 家庭版 中文版 (Build 10.0.26200) | 符合宿主运行环境假设 |
| CPU | AMD Ryzen 5 5600H with Radeon Graphics (6核 / 12线程) | 支持硬件虚拟化 |
| 物理内存 | 总计 15.36 GB / 当前空闲 2.52 GB | 内存偏紧，生产需满足内存保护门槛 |
| 显卡与驱动 | AMD Radeon(TM) Graphics (驱动版本 27.20.14064.11001) | 集成显卡，显存 512MB |
| 磁盘可用空间 | C 盘: 18.00 GB 可用 / D 盘: 54.43 GB 可用 | 满足开发存储要求 |
| 渲染组件 | Microsoft Edge WebView2 Runtime 157.0.4341.42 | 已就绪，满足 Wails 运行时要求 |
| 基础工具链 | Git 2.52.0, Node.js v22.23.4, npm 10.9.9, winget 可用 | Go、Wails、ADB、scrcpy 当前未在 PATH |

---

## 2. 模拟器安装与路径核实证据

### 2.1 注册表与文件核验

- **注册表项**：`HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Uninstall\MuMuPlayerGlobal`
  - 显示名称：`MuMuPlayer`
  - 记录版本：`5.28.0.3580`
  - 卸载路径：`"D:\Program Files\Netease\MuMuPlayer\uninstall.exe"`
- **实际文件核验**：
  - 执行 `Test-Path "D:\Program Files\Netease\MuMuPlayer"` 结果：`False`。
  - `D:\Program Files` 目录不存在，原安装目录已被移动或物理移除。
- **关联残留文件**：
  - `C:\Program Files\MuMuVMMVbox`：包含网易定制 VirtualBox 驱动核心及驱动配置 `Relationships\20260609221557600.json`。
  - `C:\Users\tieke\AppData\Roaming\Netease\MuMuPlayerGlobal`：包含历史配置 `install_config.json` 与 Android 12 镜像记录 `120.2.163.002-nemux-overseas-x64-release`。
- **本地安装包发现**：
  - 路径：`C:\Users\tieke\Desktop\giffgaff_tools\MuMu_5.0.11.exe`
  - 签名状态：有效数字签名（NetEase Interactive Entertainment Pte. Ltd.，DigiCert 颁发）。
  - 版本：MuMuPlayer 5.0 (海外版)。
- **运行进程核验**：宿主当前未运行任何 MuMu、VirtualBox 或其他模拟器进程与后台服务。

> **结论**：注册表记录存在但安装目标失效，**不能认定模拟器已就绪**。当前缺少可调用的有效 CLI 工具。

---

## 3. 模拟器接口能力矩阵 (G0 阶段状态)

依据 `docs/02-agent-implementation-plan.md` 的要求，对模拟器能力进行判定（严格区分 verified / unsupported / unverified）：

| 能力项 | 判定状态 | 现状与证据说明 |
|---|---|---|
| **discover** | unverified | 路径缺失，无法执行 CLI 列表查询命令 |
| **start** | unverified | 无法执行 CLI 命令；未指定隔离测试实例，禁止盲目启动 |
| **stop** | unverified | 无法执行 CLI 命令；未指定隔离测试实例 |
| **restart** | unverified | 待 lifecycle 服务通过 Stop + Start 组合或厂商原子命令验证 |
| **create** | unverified | G2 范围，当前未验证 |
| **clone** | unverified | G2 范围，当前未验证 |
| **reset** | unverified | G2 范围，涉及数据清除，未验证 |
| **delete** | unverified | G2 范围，涉及数据销毁，未验证 |
| **headless** | unverified | 待后续通过帧率与显卡资源对比测量 |
| **metrics** | unverified | 待后续对 MuMuVMMSVC 与主进程进行资源指标采集验证 |

---

## 4. 三大技术方案对比与推荐决策

| 方案 | 优点 | 约束与代价 | 结论 |
|---|---|---|---|
| **方案 A：MuMu 12 (MuMu 5.0 海外版/国内版)** | 宿主已有驱动与配置基线，支持现代 Android 12，提供 headless/CLI 控制潜力 | 需要完成完整安装，锁定确定版本 CLI | **推荐主方案**：建议由用户批准并执行安装/修复目标版本 |
| **方案 B：雷电模拟器 (LDPlayer 9)** | CLI（ldconsole）文档较公开，ADB 映射规则明确 | 需引入新模拟器环境，占用额外磁盘 | 备选方案：若 MuMu CLI 不满足要求时启动预案评审 |
| **方案 C：纯实体设备控制 (Pure ADB/scrcpy)** | 不依赖虚拟化和模拟器驱动，零模拟器开销 | 无法在宿主上自动化批量创建、销毁与克隆虚拟实例 | 排除作为主力方案；保留作为 ADB/scrcpy 协议测试辅助 |

---

## 5. T00 任务验收结果与准入判定 (V00)

- **V00 验收状态**：`BLOCKED`（因宿主缺少已安装的有效模拟器可执行程序，且未指定真实隔离测试实例，严禁标记 PASS 或使用 mock 冒充）。
- **阻塞原因**：
  1. `D:\Program Files\Netease\MuMuPlayer` 路径不存在，CLI 命令不可用；
  2. 尚未指定隔离测试实例 ID；
  3. Go 开发工具链与 ADB 未在系统 PATH 中。
- **进入 T01 的前置条件**：
  1. 确认/修复模拟器安装（例如安装已有的 `MuMu_5.0.11.exe` 到指定稳定路径），获取 `MuMuManager.exe` 或相关 CLI 帮助与版本输出；
  2. 由用户指定一个专用于测试的隔离实例编号；
  3. 配置或安装 Go (推荐 1.22+ 或 1.23+) 及 ADB 工具。
