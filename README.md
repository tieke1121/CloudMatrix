# CloudMatrix

Windows Android 实例管理与自动化平台。仓库包含 Wails 桌面端和 Go 后端；实机验收仍需固定版本的 MuMu CLI 与隔离测试实例。

## 阅读顺序

1. [产品与架构](docs/01-product-and-architecture.md)：产品边界、部署模型、模块职责、关键契约。
2. [开发任务](docs/02-agent-implementation-plan.md)：任务依赖、文件范围、实施步骤、交付门槛。
3. [验证计划](docs/03-validation-plan.md)：功能、故障、性能、隔离及发布验证。
4. [Agent 执行规则](AGENTS.md)：单任务工作方式、停止条件、证据要求。

方案基线：2026-11-08。已采纳用户修正：**开发电脑的配置不决定产品容量上限**。本机只承担开发和能够承受的小规模验证；生产部署到另行验证的 Windows 工作站或服务器。

## 产品阶段

| 阶段 | 可交付成果 | 禁止提前宣称 |
|---|---|---|
| G0 可行性验证 | 一个模拟器版本的真实 CLI、ADB、启动和投屏证据 | 全部模拟器通用、Windows Server 已支持 |
| G1 单宿主 MVP | 发现、启停、独立投屏、安装 APK、任务和日志、状态恢复 | 实时视频墙、无人值守长期稳定、多宿主 |
| G2 矩阵与可靠性 | 实时监控墙、可选同步、模板能力、受控恢复、长稳测试 | 固定硬件下必然支持 20 台、保证无网络泄漏 |
| G3 远程与生产 | Windows Host Agent、远程鉴权、网络隔离、发布与回滚 | Linux 模拟器兼容、绕过第三方平台验证 |

G0/G1 是可立即交给 Agent 逐任务执行的开发范围。G2/G3 有明确调查、验证和设计任务；通过各自准入门槛后，才能进入实现，不能把路线图直接当成编码许可。

## 构建与配置

```powershell
npm --prefix frontend ci
pwsh -NoProfile -File scripts/package.ps1
```

打包成功后，从输出的新发行目录运行 `bin/CloudMatrix.exe`。旧版 `build/bin` 不会被覆盖。

- `CLOUDMATRIX_CONFIG` 可指定 JSON 配置文件。ADB、Android SDK Build-Tools 的 `aapt2` 和 scrcpy 默认从 PATH 解析，也可通过 `adb.executable`、`aapt2.executable` 与 `scrcpy.executable` 指定。
- 完成 T00 后，在配置中填写已验证 CLI 的 `provider.executable` 绝对路径和 `provider.verified_cli_sha256`。未配置时，刷新会明确提示缺少工具；摘要不匹配时拒绝启动。
- 新安装默认将数据放在 `%APPDATA%\CloudMatrix\data`。旧库仍保留在原目录；要继续使用它，将 `data_dir` 指向旧库所在目录，先关闭其他正在使用该库的 CloudMatrix 进程。

## 状态说明

- 桌面端接入发现、启停、独立投屏，以及限定包名 `com.haoyisheng.mobile.app` 的好医生 APK 校验、安装后版本确认、启动/停止和人工验证码登录状态记录。不会读取验证码，不保存完整手机号、密码、Cookie 或 Token。
- 好医生真实安装与验证码登录尚未在隔离模拟器上验收；没有真实证据前保持 `NOT_RUN`。
- Host Agent 只提供本机探活；视频墙、同步、模板、自愈和网络隔离仍是未接入桌面入口的原型。
- 文档中的验收阈值是产品目标；只有附带硬件、负载和测量记录的结果才算通过。
- 已连接的实体手机可辅助测试 ADB/scrcpy，但不能代替模拟器生命周期测试。
- 不把 REA、用户电脑上的绝对工具路径或现有账号作为运行产品的依赖。
