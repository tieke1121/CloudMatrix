# 第三方开源组件与商业授权清点声明

CloudMatrix 平台严格遵守开源软件许可协议与商业知识产权合规要求。以下为核心第三方组件许可清单：

---

## 1. 开源运行依赖与工具链

| 组件名称 | 许可协议 | 来源与作用 | 再分发合规策略 |
|---|---|---|---|
| **Android SDK Platform-Tools (ADB)** | Apache License 2.0 | 设备连接与命令控制 | 随发布包包含独立预编译二进制，保留 NOTICE 与 LICENSE 文件 |
| **scrcpy / scrcpy-server** | Apache License 2.0 | 高性能视频流与输入桥接 | 保留原版权声明，不修改 server jar 核心签名 |
| **modernc.org/sqlite** | BSD 3-Clause | 纯 Go 内嵌 SQLite 引擎 | 静态编译进主程序，无额外运行时 C 运行时库依赖 |
| **Vue 3 / Vite** | MIT License | 前端响应式界面与构建工具 | 生产代码打包已抽离 LICENSE 注释 |
| **Element Plus** | MIT License | 桌面管理 UI 组件库 | 遵循 MIT 许可规范 |

---

## 2. 商业模拟器与专有软件边界声明

- **网易 MuMu 模拟器 / 雷电模拟器 / 逍遥模拟器**：
  - 均属于第三方专有商业软件；
  - **严禁事项**：CloudMatrix **绝对不随产品安装包打包、再分发任何商业模拟器安装包、内核驱动或镜像磁盘**；
  - **接入方式**：全部模拟器必须由最终用户自行下载安装并遵守对应商业最终用户许可协议 (EULA)；
  - **调用边界**：CloudMatrix 仅通过对应模拟器官方公开的命令行接口 (CLI) 和标准 ADB 端口进行只读探测与生命周期编排。
