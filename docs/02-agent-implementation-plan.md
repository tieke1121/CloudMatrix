# CloudMatrix Implementation Plan

> For agentic workers：按 AGENTS.md 逐任务执行；每次只领取一个任务。若环境有计划执行技能可辅助使用，但不能把不存在的技能当作阻塞条件。不得跳过前置任务或把后续路线图一次性实现。

**Goal:** 先交付经过真实模拟器验证的单宿主 Windows 管理平台，再通过独立门槛扩展矩阵、远程与生产能力。

**Architecture:** Go 应用服务隔离厂商适配器与 UI，G1 本机进程内调用；实例、任务与媒体会话有独立状态和清理边界。HostID 和版本化契约为后续远程 Host Agent 保留扩展点，不提前部署分布式组件。

**Tech Stack:** Go、Wails v2、Vue 3、TypeScript、Vite、Element Plus、SQLite、厂商 CLI、官方 ADB、固定版本 scrcpy。

---

## 执行说明

以下文件路径相对仓库根目录，属于计划创建的文件；当前尚不存在。每个复选步骤是独立操作。工程实现由开发 Agent 在领取任务后补齐，本文不冒充已可运行的源码。

一个 T 任务是一次可审查交付，可由多个小提交组成。先检查失败行为，再实现最小闭环，最后报告测试结果。真实设备操作只能在用户指定的测试实例进行。

状态统一为 NOT_STARTED / IN_PROGRESS / BLOCKED / DONE。已有实现和自动化测试的任务可记录为 IN_PROGRESS；缺少真实模拟器、隔离实例或目标宿主的验收项保留 BLOCKED。证据目录为 artifacts/verification/<run-id>/，其中 run-id 用 UTC 时间和随机后缀生成，避免并行覆盖。证据不得提交账号、身份信息或代理密码。

全局检查入口由 T01/T10 创建：

```powershell
go test ./... -count=1
npm --prefix frontend run typecheck
npm --prefix frontend run test:unit -- --run
npm --prefix frontend run build
wails build -platform windows/amd64
```

首期后端验证不依赖未安装的模拟器；集成验证显式读取测试实例配置。缺环境时退出非零并标记 BLOCKED，不能返回通过。Go race 检查仅在具备所需编译工具链的 runner 执行，不能把未运行写成通过。

## 依赖图与阶段门槛

```text
T00 → T01 → T02 → T03 → T04 → T05 → T06
                                   └→ T07 → T08
                 T03 + T05 + T07 → T09
          T05 + T06 + T07 + T08 + T09 → T10 → T11
G1通过 → T12、T13 → T14
G1通过 + T09 → T15
G1通过 → T16、T17
T12~T17 的实际发布范围验收完成 → T18
```

顺序优先，不要求使用多 Agent。同一目录存在其他 Agent 工作时必须协调文件归属；禁止同时改协议、迁移和锁文件。

## G0：可行性与工程基线

### T00 — 确认宿主与模拟器适配能力

依赖：无。范围：docs/evidence/provider-capabilities.md（本任务生成的必要证据）、artifacts/verification/<run-id>/provider/。

- [ ] 核实模拟器真实安装路径、版本、厂商签名、支持系统和商业使用/分发边界；注册表路径失效则报告，不能认定已经安装。
- [ ] 只读获取该版本 CLI 帮助与实例列表；记录原命令、退出码、原始输出、解析后的实例标识及端口来源。
- [ ] 建立能力表：discover/start/stop/restart/create/clone/reset/delete/headless/metrics。每项只能是 verified、unsupported 或 unverified，并附证据。
- [ ] 确认启动、停止和克隆命令的作用范围；实机行为验证需用户指定隔离测试实例，未指定时保留 BLOCKED。
- [ ] 比较已有三个方案：MuMu 适配、另选厂商、仅实体设备控制。推荐一个满足最低能力的模拟器版本，冻结选择后才进入 T01。

验收 V00：discover/start/stop 与动态发现 ADB 地址有证据；若启动和停止尚未测试，G0 不通过。禁止“参考网上命令推测可用”。

### T01 — 建立可复现工程与工具版本锁

依赖：T00。创建：go.mod、go.sum、main.go、app.go、wails.json、frontend/package.json、frontend/package-lock.json、frontend/src/main.ts、frontend/src/App.vue、scripts/check.ps1、tools/versions.json。

- [ ] 选择受支持且可取得的 Go、Wails v2、Node、ADB、scrcpy 精确版本；记录下载来源、校验值、版本输出、SQLite 驱动兼容性和 WebView2 状态。
- [ ] 从对应 Wails 官方模板生成最小 Vue + TS 工程，替换示例页面为空设备列表；引入 Element Plus。
- [ ] 添加 typecheck、test:unit、build 脚本；后续测试无用例不能被配置为成功。
- [ ] scripts/check.ps1 顺序执行 Go 测试、前端检查和 Wails 构建；任一子命令失败立即原样返回非零退出码。
- [ ] 验证干净依赖安装、开发启动和 Release 构建；工具本地路径不写入源码常量。

验收 V01：干净 Windows 验证环境依照版本锁可构建并启动空页面；不存在模拟器时仍可启动并报告未配置。安装开发工具需纳入用户批准的安装范围。

### T02 — 进程执行与取消基础

依赖：T01。创建：internal/execx/runner.go、runner_windows.go、runner_test.go、testdata/child/main.go。

- [ ] 定义 Run(ctx, executable, args, timeout, outputLimit)；结果含退出码、截断标志和分类错误；executable 必须为解析后的受控工具路径。
- [ ] 先写子进程夹具：成功、退出 7、持续输出、挂起、派生子进程、路径含空格及中文。
- [ ] 实现无 Shell 拼接的启动、取消、Windows 子进程归属与清理；不按进程名终止无关进程。
- [ ] stdout/stderr 各限制 1 MiB，满后持续排空或受控终止并报告 OUTPUT_LIMIT，不能阻塞管道或无界累积。
- [ ] 运行 `go test ./internal/execx -count=1`，核对超时和取消后 5 秒内无本任务残留子进程。

验收 V02：带 `&`、引号的参数作为字面参数传入，不启动第二个命令；取消不会误杀独立启动的同名夹具。

## G1：单宿主可用产品

### T03 — 数据模型、配置与持久化

依赖：T02。创建：internal/domain/{instance,task,error,capability}.go、internal/config/{config,config_test}.go、internal/store/{sqlite,migrate,store_test}.go、internal/store/migrations/001_init.sql。

- [ ] 根据架构文档建立 InstanceID/HostID/ProviderInstanceID、期望状态与观测状态、分目标任务结果。
- [ ] 配置解析校验范围；不合法路径、负数并发、超大超时必须拒绝并指出配置键。
- [ ] 实现事务迁移、单写者策略、可配置 busy timeout、查询取消；迁移失败保持原库可恢复。
- [ ] 为实例唯一性、任务幂等键、重启恢复、数据库锁冲突、迁移失败添加测试。
- [ ] 运行 `go test ./internal/config ./internal/store -count=1`；检查数据库与日志不保存凭据明文。

验收 V03：同一实例不能因刷新列表重复插入；数据库版本高于程序支持时拒绝写入，而不是重建数据库。

### T04 — 模拟器发现与 ADB 就绪探测

依赖：T03、T00。创建：internal/provider/provider.go、internal/provider/mumu/{provider,parse,parse_test}.go、internal/adb/{client,readiness,client_test}.go、internal/provider/mumu/testdata/*.json。

- [ ] 定义最小 Provider 接口：Capabilities、Discover、Start、Stop；Restart 首期由 lifecycle 组合，不要求厂商必须有原子重启命令。
- [ ] 用 T00 的脱敏输出建立解析夹具，覆盖空列表、重复 ID、缺字段、未知字段、非零退出和本地化输出。
- [ ] 用厂商标识关联实际 ADB serial；不得假定 5555+2N，也不得把当前所有实体手机自动纳管。
- [ ] 实现 ADB device/offline/unauthorized 分类，以及系统启动完成和包管理可用检查。
- [ ] 运行 `go test ./internal/provider/... ./internal/adb -count=1`，再对指定测试实例只读验证。

验收 V04：本机同时有实体手机和模拟器时，仅纳管选定目标；ADB 连通但系统未就绪时不能出现 ready。

### T05 — 生命周期状态机与并发准入

依赖：T04。创建：internal/lifecycle/{service,state,admission,service_test}.go。

- [ ] 建立状态转换表并用表驱动测试验证；error/unknown 不自动转换为 stopped。
- [ ] 实现实例级写锁、全局启动队列、幂等 Start、带超时的就绪探测。
- [ ] 实现 Stop 和组合 Restart；进程取消后重新确认实际实例状态。
- [ ] 资源准入读取部署配置和当前宿主余量；申请和释放配额必须原子化，失败路径释放预留。
- [ ] 运行 `go test ./internal/lifecycle -count=1`；真实测试连续启停 10 次并确认无重复实例、错误绑定和遗留预留。

验收 V05：连续点击启动 20 次最多产生一次厂商启动；启动中收到停止不会最终错误展示 ready。

### T06 — 独立投屏会话

依赖：T05。创建：internal/stream/{manager,scrcpy,manager_test}.go。

- [ ] 固定经过验证的 scrcpy 版本，指定确切 serial；参数必须使用当前工具帮助确认。
- [ ] 每实例至多一个默认交互会话；再次打开聚焦或返回已有会话。
- [ ] 观测 scrcpy 退出并回收自己的进程、转发/会话资源；失败不结束模拟器。
- [ ] 测试重复打开、关闭、ADB 断开、编码器不可用和路径空格。
- [ ] 运行 `go test ./internal/stream -count=1`；人工验证点击、拖动、返回、中文输入兼容性与剪贴板行为。

验收 V06：关闭投屏不关闭实例；此任务只交付独立窗口，不宣称实现嵌入式视频墙。

### T07 — 有界任务队列与 APK 分发

依赖：T05、T03。创建：internal/task/{queue,install,recovery,queue_test,install_test}.go。

- [ ] 创建批量任务时固定目标集合，保存逐实例 task_target；默认并发 2 可配置。
- [ ] 校验本地 APK 存在、是普通文件、文件大小在配置范围内，并保存摘要；不根据扩展名认定安装必然可行。
- [ ] 首期支持单 APK；APKS/XAPK/拆分包明确拒绝并解释，不能只安装其中一个文件后报成功。
- [ ] 解析 ADB 安装失败原因；超时先核对包状态，不自动重复安装或覆盖应用数据。
- [ ] 覆盖三个目标一成功一失败一离线、队列取消、任务进程中断及 UI 重试幂等；运行 `go test ./internal/task -count=1`。

验收 V07：批量结果准确显示部分失败；文件在校验后变化则拒绝或使用已校验的稳定副本，不能记录错误摘要。

### T08 — 受控命令控制台

依赖：T07、T02。创建：internal/task/{command,command_test}.go、internal/observe/{redact,redact_test}.go。

- [ ] 提供常用只读诊断动作与显式高级命令模式；执行前显示目标集合和命令。
- [ ] 高级命令明确属于 Android shell 语法，不能经过宿主 PowerShell/cmd 二次解释。
- [ ] 设置超时、并发和输出上限，执行结果与目标逐一关联。
- [ ] 敏感字段脱敏；用户主动写入命令的秘密不能持久化到日志；原始高级命令审计默认保存摘要和类型而非全文。
- [ ] 运行 `go test ./internal/task ./internal/observe -count=1`；验证非零退出、无限输出和取消后任务结束。

验收 V08：一个目标失败不阻塞其他目标；停止命令不被错误显示为成功。

### T09 — 状态恢复、事件与指标

依赖：T03、T05、T07。创建：internal/observe/{metrics,events,health,health_test}.go、internal/lifecycle/reconcile.go。

- [ ] 程序启动后将旧 running 任务标为 interrupted，重新发现实例与核对厂商状态。
- [ ] 指标含采样时间、归属与是否缺失；无法关联进程时显示 unknown，而非 CPU=0。
- [ ] 事件流包含 sequence、InstanceID/TaskID；UI 丢事件可拉取快照重建状态。
- [ ] 健康检查连续失败 3 次才标记异常，恢复需连续成功 2 次；G1 只提示，不自动重启实例。
- [ ] 运行 `go test ./internal/observe ./internal/lifecycle ./internal/task -count=1`；模拟桌面崩溃后重新打开，验证状态与真实实例一致。

验收 V09：旧日志/任务有保留上限；无法确认状态时展示 unknown 和最后成功观测时间。

### T10 — 桌面操作闭环与集成验证入口

依赖：T05~T09。创建：internal/app/{service,dto}.go、frontend/src/{api,types}/、frontend/src/views/{Instances,Tasks,Settings}.vue、frontend/src/components/InstanceCard.vue、frontend/src/**/*.test.ts、scripts/verify-integration.ps1、tests/integration/。

- [ ] 绑定类型化应用接口；UI 只表达意图，不接收任意可执行文件路径后直接运行。
- [ ] 完成配置校验、实例列表、多选、启动/停止、打开投屏、APK 分发、任务详情和可取消操作。
- [ ] 添加 loading、empty、error、partial success、离线等状态；pending 时禁用重复写操作。
- [ ] 前端测试覆盖目标选择、状态更新、错误展示、取消反馈和事件乱序；不得靠定时假数据填充设备列表。
- [ ] 集成脚本只接受显式传入的测试实例 ID，缺失时拒绝；生成结构化验证结果和命令退出码。
- [ ] 运行全局检查命令；为隔离测试实例执行 `pwsh -NoProfile -File scripts/verify-integration.ps1 -InstanceId '<测试实例ID>' -Serial '<已核对的ADB序列号>' -AdbPreflightOnly`，然后独立记录真实生命周期和桌面操作结果。ADB 预检不替代全流程验收。

验收 V10：用户在桌面 UI 完成“发现→启动→就绪→投屏→关闭投屏→停止”；错误与 CLI/ADB 真实结果一致。

### T11 — 基线容量与稳定性报告

依赖：T10。创建：scripts/benchmark.ps1、tests/performance/、docs/evidence/capacity-baseline.md。

- [ ] 固定宿主、驱动、Android、模拟器、App、分辨率和负载；定义资源采样范围。
- [ ] 从一个实例测起，按可用资源递增；遇准入阈值停止，不能为凑 10/20 台使宿主失去响应。
- [ ] 分别测空闲、固定动画、实际授权 App、投屏开/关；保存原始数据和 P50/P95。
- [ ] 执行 2 小时单宿主 MVP 稳定性与故障恢复验证；生产 168 小时验证属于 T17。
- [ ] 根据 docs/03-validation-plan.md 生成容量等级和瓶颈报告。

验收 V11：有可复现实测报告，而非把开发机数据推算成服务器容量。G1 的发布功能通过 V00~V11 才可称 MVP 完成。

## G2/G3：逐子项目推进，不直接从路线图编码

下列任务先交付明确的 PoC、设计和验收证据；通过后再为该子项目编写同等颗粒度的实现任务。这样避免 Agent 将未证实的底层能力写成虚假实现。

### T12 — 模板与受控实例创建

依赖：G1。计划文件：docs/design/template-lifecycle.md、internal/provider/mumu/template.go、internal/template/、tests/integration/template_test.go。

- [ ] 用测试实例验证厂商创建、冷克隆、导入导出、重置和删除，每项记录是否支持与授权限制。
- [x] 定义模板摘要、版本、引用计数、来源、最小磁盘余量；基底变更不能破坏现有实例 (见 internal/template/)。
- [ ] 验证两个新实例的测试文件、测试账号状态互不影响；黄金模板禁止真实账号数据。
- [x] 制定破坏操作二次确认、超时后核对、创建失败清理与磁盘链保护方案。
- [ ] 提交真实 V12 证据；不支持差分克隆时明确降级为完整克隆并重新测量。

### T13 — 真正的视频墙与后台资源策略

依赖：G1。计划文件：docs/design/media-pipeline.md、internal/media/、frontend/src/components/VideoTile.vue、tests/performance/media/。

- [ ] 对比原生渲染窗口与 WebView 解码展示两条路线；做真实 4 路同负载 PoC，测延迟、CPU、GPU、拷贝和释放。
- [ ] 验证固定 scrcpy 协议、断线恢复、硬解不可用降级、窗口遮挡、DPI、旋转、RDP 断连。
- [ ] 证明后台降低采集帧率具体节约了哪些资源；区分视频暂停、实例暂停、窗口隐藏和真正无头。
- [ ] 从 4 路逐步到 9/16 路，定义自适应清晰度和后台流数量；不在 UI 线程解码。
- [ ] 交付 V13 和选型决策后才编写生产媒体模块任务；目前保留 G1 独立窗口方案。


### T14 — 多屏输入同步

依赖：T13 的稳定交互通道。计划文件：docs/design/sync-engine.md、internal/sync/、tests/integration/sync_test.go。

- [x] 定义坐标有效区域、方向映射、序列号、按键释放和用户急停语义 (见 docs/design/sync-engine.md 与 internal/sync/)。
- [ ] 用同一测试 App 验证同分辨率、不同宽高比、旋转、目标断线及弹窗漂移。
- [ ] 分别记录发送时间差和可观测执行时间差；跨机器指标需要时钟同步与误差说明。
- [ ] 验证慢目标不会拖死全部目标，过期输入不堆积重放，急停 1 秒内停止新事件下发。
- [ ] 交付 V14 和具体实现任务；不同页面的输入广播不能被标成业务步骤全部成功。

### T15 — 受限自愈守护

依赖：G1、T09。计划文件：docs/design/recovery-policy.md、internal/watchdog/、tests/integration/recovery_test.go。

- [x] 区分 UI 退出、投屏退出、ADB 失联、实例退出和宿主资源枯竭 (见 docs/design/recovery-policy.md 与 internal/watchdog/)。
- [x] 定义每实例 10 分钟内最多自动重启 2 次的初始预算、冷却时间和用户关闭开关；用户主动停止不得触发恢复。
- [x] 同步/破坏任务期间禁止盲目重启；恢复只用于已授权托管实例。
- [ ] 注入连续故障、主机重启、休眠和磁盘满，验证不会形成重启风暴。
- [ ] 交付 V15 与实现任务，恢复失败转人工处理，不反复消耗资源。


### T16 — 实例网络隔离 PoC

依赖：G1。计划文件：docs/design/network-isolation.md、internal/network/、tests/integration/network/。

- [x] 列明支持的代理类型、认证方式、TCP/UDP/DNS/IPv6范围；不支持的协议必须显式阻断或声明不覆盖。
- [ ] 在可控测试端点验证每实例出口、DNS 请求与代理失效行为；现有探针夹具仅验证状态转换。
- [ ] 检查开机前后、VPN 崩溃、休眠恢复、网络切换和证书过期时的 fail-closed 行为。
- [x] 设计凭据存储、轮换和脱敏；禁止把代理用户名密码写入可见命令行。
- [x] 交付 V16 与实现计划；未验证全部声明的路径前，UI 不展示“完全隔离”。


### T17 — 高性能 Windows 宿主与远程 Host Agent

依赖：G1；远程媒体还依赖 T13。计划文件：docs/design/remote-host.md、cmd/cloudmatrix-host/、internal/transport/、api/、tests/integration/remote/。

- [ ] 在目标服务器验证模拟器、虚拟化、GPU 驱动和会话条件；记录 Windows Server/RDP/嵌套虚拟化支持边界。
- [x] 定义版本化 RPC、HostID、机器身份、TLS、权限、重连与幂等；媒体认证与控制鉴权一致。
- [x] 模拟断网和请求响应丢失：重新查询同一 task_id，不重新执行有副作用命令。
- [ ] 按目标容量等级做 168 小时长稳实验；至少两个真实宿主验证远程能力，单机多进程不替代 (缺少第二台物理机环境，如实标记 BLOCKED)。
- [ ] 完成 V17 真实任务闭环。Host Agent 已可独立编译，仅支持回环探活；当前未接入远程任务执行器。


### T18 — 分发、升级与恢复

依赖：所发布范围对应任务通过。计划文件：docs/release/runbook.md、scripts/package.ps1、scripts/verify-release.ps1、licenses/、tests/integration/upgrade/。

- [x] 清点第三方许可和再分发条件；模拟器和镜像默认由用户安装，未获许可不随产品分发 (见 licenses/THIRD-PARTY-LICENSES.md)。
- [x] 发布包包含版本清单、文件校验、工具兼容范围、升级和回滚步骤；签名状态如实记录 (见 build/release/manifest.json 与 docs/release/runbook.md)。
- [ ] 验证完整升级前备份与回滚流程。现有测试覆盖 SQLite 冷备份和高版本库拒绝写入。
- [ ] 在无开发环境的干净 Windows 宿主执行安装、启动、卸载与重新安装，卸载保留用户实例数据。
- [ ] 运行 V18，列明实际支持的 OS、模拟器、并发等级和已知限制；发行文件校验仅证明文件集合和摘要符合清单。


## 任务交接模板

```text
任务：Txx；状态：DONE / BLOCKED
前置证据：运行记录目录或提交
实际修改文件：逐项列出
验证：命令、退出码、关键结果、真实/模拟环境
失败和未运行项：原因及影响
数据/进程清理：本任务创建了什么、已清理什么
下一步：仅指出满足依赖的下一任务，不自动扩展范围
```

完成计划后，建议先由用户审阅架构和阶段边界，再把 README 中的 T00 指令交给实现 Agent。本轮不初始化工程、不提交空实现、不启动批量设备操作。
