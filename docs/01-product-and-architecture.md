# CloudMatrix 产品与架构方案

版本：0.1；日期：2026-11-08；状态：供用户审阅的开发基线。

## 1. 产品定位与环境假设

统一管理 Windows 宿主机上的 Android 模拟器，提供实例生命周期、画面查看、交互、批量安装、受控任务及观测能力，用于授权的应用测试和设备管理。

开发环境与部署环境分离：本机只用于开发和小规模实验。生产容量由目标宿主机的 CPU、GPU、RAM、存储、模拟器版本、Android 版本和实际业务负载共同决定。

首期部署模型为**单 Windows 宿主机、交互式桌面用户运行**。Windows Server、云服务器、RDP 断连后的 GPU 行为、嵌套虚拟化与 Session 0 运行全部属于单独的兼容性验收项。不能从 Windows 11 可运行推导 Server 可运行。Linux 宿主不在当前实现范围。

## 2. 原方案需求处理

| 原需求 | 处理 | 对应任务 |
|---|---|---|
| 多实例启停/重启/状态 | G1 实现；首次先发现和接管用户选定实例 | T00、T04、T05 |
| 创建/克隆/重置/删除 | 使用厂商已验证接口；G2 分能力开放 | T12 |
| 实时视频墙/独立大屏 | G1 独立 scrcpy 窗口；G2 真正实时视频墙 | T06、T13 |
| 多屏输入同步 | G2 明确目标集合、坐标变换、漂移检测和急停 | T14 |
| 独立代理、存储隔离 | 存储边界 G1；全流量网络隔离 G3 专项 | T03、T12、T16 |
| 好医生 APK 安装 | G1 限定官方包名、校验版本、逐实例安装后确认 | T07 |
| 验证码登录 | 用户在投屏窗口内操作；仅记录状态和可选手机号后四位，不读取验证码 | T07 |
| 通用 APK 分发、Shell 控制台 | 不进入当前产品范围 | - |
| 后台低功耗/无头 | 测量后逐项开放；不把降视频帧率等同于虚拟机休眠 | T11、T13 |
| 设备指纹随机伪装 | 不做规避第三方识别的身份伪装；仅保留厂商支持的可复现测试配置 | T00、T12 |
| 异常自愈 | G1 检测和显式恢复；G2 受限自动恢复 | T09、T15 |
| 10~20 台、24×7 | 作为目标部署环境的容量等级和长稳验收，不是本机门槛 | T11、T17 |
| 商业分发 | 核对模拟器、scrcpy、编解码库和镜像授权后再决定随包分发 | T00、T18 |

新增或删除用户明确要求的功能必须更新此映射并说明原因，不能悄悄宣称完整原方案已实现。

## 3. 技术方案比较与选定基线

| 路线 | 优点 | 代价 | 决策 |
|---|---|---|---|
| Go + Wails + Vue | 符合开发者技术栈，调度逻辑可独立测试 | 多路原生视频嵌入仍需专项工作 | 主方案 |
| C# + WPF | Windows 原生窗口和图形接口整合便利 | 更换主要技术栈、Go 代码复用减少 | 视频 PoC 失败后再评审，不双线开发 |
| 从零维护 Android/虚拟机内核 | 内核能力可控 | 驱动、镜像、GPU、兼容和升级成本大 | 排除首期 |

首期：Go、Wails v2、Vue 3 + TypeScript + Vite + Element Plus、SQLite。Wails v2 是本方案的保守选型，并非宣称 v3 未发布；升级为 v3 必须通过独立构建与窗口测试。T01 锁定具体依赖版本，不在生产安装流程使用浮动 latest。

数据库采用 database/sql + modernc.org/sqlite，避免为 SQLite 引入 CGO 构建前置；若 T01 的固定 Go/驱动版本兼容性验证失败，先评审替代，不混用 GORM 与另一套迁移系统。

模拟器先选一个具体 MuMu 版本，是否能选用由 T00 的安装、CLI 和授权证据决定。首期通过参数数组调用官方 ADB 可执行文件；不重新实现 ADB 协议，也不使用 iOS 的 gwda 库。

## 4. 进程与模块架构

G1：一个桌面应用进程；Go 应用服务直接调用本机适配器；SQLite 为状态存储；scrcpy 是受管理的独立子进程。UI 不直接执行 CLI，不读取代理密码，不绑定物理端口规则。

模块：

| 目录 | 职责 |
|---|---|
| internal/domain | 实例、状态、任务、错误码、能力定义 |
| internal/execx | 参数化进程执行、超时、取消、输出边界、进程归属 |
| internal/provider/mumu | 唯一的首期模拟器适配器 |
| internal/adb | 指定设备的 ADB 查询、就绪检查和操作 |
| internal/lifecycle | 并发互斥、期望/观测状态、启动与停止 |
| internal/store | SQLite 迁移、实例/任务/事件持久化 |
| internal/stream | scrcpy 子进程会话与后续视频适配 |
| internal/task | 有界队列、分实例执行结果、恢复语义 |
| internal/observe | 指标、事件、脱敏、健康探测 |
| internal/app | 应用用例及 Wails 绑定；不写厂商命令细节 |
| frontend/src | Vue 页面、类型化视图模型和交互 |
| tests/integration | 需要真实测试实例的验收 |

G3：在宿主节点增加 cloudmatrix-host.exe。桌面通过版本化 RPC 访问 Host Agent；媒体走独立认证通道。优先复用内部应用用例，不在 G1 提前部署 gRPC 服务、消息队列和控制平面数据库。

HostID 从 G1 数据模型保留，默认 local；InstanceID 使用内部 UUID，ProviderInstanceID 单独存储，不能使用列表下标或端口作为永久主键。

## 5. 状态与运行契约

生命周期：discovered → stopped → starting → booting → ready → stopping → stopped。失败转换到 error；宿主/连接无法确定时是 unknown，不擅自当作 stopped。desired_state 与 observed_state 分开记录。

ready 同时要求：厂商报告实例运行、ADB 已授权可达、Android 启动完成、包管理查询成功。视频是否可用单独记录 stream_state，投屏失败不必把正在运行的实例标成关机。

- 同一实例的写操作串行；不同实例受全局并发限制。
- Start(stopped) 创建一次启动任务；Start(starting/booting/ready) 返回已有任务或幂等结果。
- unknown 状态先刷新状态，不能盲目新建实例或杀进程。
- 停止按钮取消启动后，先确认厂商实际状态，再决定是否调用已验证的停止接口。
- 重启桌面应用后发现并恢复状态；不能把数据库里的 ready 当作实时状态。
- 进程取消不能只取消 Go goroutine；必须回收本次调用拥有的子进程。不能按通用进程名杀其他任务。

初始超时：普通查询 10 秒、启动就绪 180 秒、停止 60 秒、APK 安装 180 秒、Shell 默认 30 秒且最大 300 秒。用户可在合法范围调整。查询允许有限退避；安装、重置等有副作用操作超时后先查询实际结果，禁止无条件重放。

## 6. 配置、任务与错误契约

配置键：schema_version、data_dir、provider.type、provider.executable、adb.executable、aapt2.executable、scrcpy.executable、limits.start_concurrency、limits.task_concurrency、limits.max_running_instances、timeouts、retention。

默认启动并发 1、普通任务并发 2；max_running_instances 来自操作者选定的部署配置，不把默认值当作物理容量结论。所有数字做类型与范围校验。工具路径配置化；启动前读取版本并校验支持范围。

数据表：hosts、instances、tasks、task_targets、events、app_sessions、schema_migrations。app_sessions 只保存登录状态和可选的掩码账号标记；不保存手机号、短信验证码、密码、Cookie 或 Token。代理凭据不存明文；后续由 Windows Credential Manager 或 DPAPI 保护，数据库仅保存引用。事务只覆盖状态写入，不把外部进程执行放进长事务。

任务状态：queued/running/succeeded/failed/cancelling/cancelled/interrupted。每个 task_target 有独立状态、退出码、错误码、时间戳。批量任务必须展示部分成功，不能用一个绿色提示掩盖失败目标。

错误码：TOOL_MISSING、UNSUPPORTED_VERSION、PROVIDER_UNAVAILABLE、INSTANCE_NOT_FOUND、ADB_UNAUTHORIZED、ADB_OFFLINE、BOOT_TIMEOUT、RESOURCE_LIMIT、PROCESS_TIMEOUT、PROCESS_EXIT、OUTPUT_LIMIT、CANCELLED、CONFLICT、INVALID_ARGUMENT、STORAGE_FAILURE。错误包含可读说明和可重试性，不包含凭据。

安装使用 task_id；同一请求只允许一个活跃执行。入队前使用 aapt2 读取 APK manifest，包名必须精确等于 `com.haoyisheng.mobile.app`；ADB 返回安装成功后还必须由设备包管理器确认相同 versionCode。UI 重连可重新查询结果；客户端失去连接不等同于服务端已取消。

## 7. 视频、交互与模板

G1 只提供独立 scrcpy 窗口，列表采用状态卡片，必要时用户手动截图刷新。不得把截图轮询命名为实时视频墙。固定 scrcpy 客户端/服务端版本组合，关闭非必要音频。

G2 视频墙先以 4 路验证解码、缩放、DPI、窗口遮挡、资源回收及 GPU 内存，再测 9/16 路。硬解失败必须报告降级和容量影响。当前协议是否支持运行时改帧率必须实测；不支持时受控重建流，不能伪造已生效的配置。

同步输入包含 sequence、capture_time、目标集合快照、归一化坐标、源方向与有效显示矩形。映射扣除黑边，处理旋转和不同宽高比。拖动必须有完整 down/move/up；连接丢失要释放按键；队列有界。不同页面不是同步成功，必须允许用户随时排除实例和急停。

模板只使用厂商支持的冷克隆/导入导出，能力未验证则按钮禁用并解释。模板需不含真实账号、Token 或个人资料。模板版本不可变、记录校验值和引用关系；不能删除仍被使用的基底。分开测量模板复制耗时、实例创建耗时和启动就绪耗时。

## 8. 网络与远程部署

G1 网络只展示连通性，不承诺全协议隔离。G3 使用独立网络 PoC 检验 VPN/TUN/代理链路，明确是否支持 TCP、UDP、DNS、IPv6及故障时阻断。ADB 端口转发本身不是全协议路由方案。

Windows 高性能服务器的准入：虚拟化能力、模拟器支持的系统版本、GPU/驱动、交互会话或受支持的无头模式、磁盘容量和必要授权。不存在“服务器一定更适合”的默认结论；需通过 T17。

远程 Host Agent 默认不暴露公网 ADB；RPC 使用双向认证或等效机器身份、TLS、权限分离和命令审计。UI 断线不自动取消已确认运行的任务。跨宿主复制数据库不能当成实例迁移。灾难恢复必须包含配置、数据、模板依赖和凭据重绑定步骤。

## 9. 证据与来源

本机只读检查：CloudMatrix 当时为空目录；PATH 未找到 Go/Wails；注册表有 MuMuPlayer 5.28.0.3580，但记录的 D:\Program Files\Netease\MuMuPlayer 路径不存在。**这是待核实的注册记录，不是可用能力证明。** 本方案没有启动或操作任何模拟器实例。

官方参考（2026-11-08 查阅；实现时以本机固定版本帮助和实测为准）：

- [Wails v2 安装与 WebView2 前置](https://v2.wails.io/docs/v2.15.0/gettingstarted/installation/)
- [scrcpy 协议与开发说明](https://github.com/Genymobile/scrcpy/blob/master/doc/develop.md)
- [scrcpy 视频参数](https://github.com/Genymobile/scrcpy/blob/master/doc/video.md)
- [Android ANDROID_ID 范围](https://developer.android.com/reference/android/provider/Settings.Secure#ANDROID_ID)
- [Android VPN 生命周期与阻断选项](https://developer.android.com/develop/connectivity/vpn)
- [Android HTTP 代理定义](https://developer.android.com/reference/android/net/ProxyInfo)
- [gwda 是 iOS 客户端](https://github.com/electricbubble/gwda)

MuMu 官方帮助链接本次未成功读取，不能据此写死 MuMuManager.exe 或 mumu-cli.exe 的命令格式；T00 必须采集目标安装版本自己的帮助输出。
