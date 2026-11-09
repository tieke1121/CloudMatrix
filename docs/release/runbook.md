# CloudMatrix 发布、升级、回滚与运维手册

当前发行包面向 Windows amd64 开发验证。使用桌面端需要 WebView2；MuMu、ADB、Android SDK Build-Tools（提供 aapt2）和 scrcpy 由使用者独立安装和配置。真实模拟器端到端、好医生安装/登录、Windows Server、并发容量和长期稳定性尚未验收。Host Agent 仅提供回环探活。

## 打包与安装

1. 在项目根目录运行 npm 依赖安装，再运行 scripts/package.ps1。默认生成独立的 build/releases 子目录。
2. 对该目录运行 verify-release.ps1，并显式传入 ReleaseDir。校验检查文件集合、大小和 SHA256；不会启动程序。
3. 从校验通过的发行目录运行 install.ps1。默认安装到 Program Files 下，若无写权限可用 InstallPath 指定其他位置。安装器只复制清单列出的发行文件。
4. 运行 bin/CloudMatrix.exe。工具配置通过 CLOUDMATRIX_CONFIG 指定，默认数据位于 %APPDATA%\CloudMatrix\data。旧目录里的库不会被自动迁移；继续使用旧库时，先关闭旧进程，再通过 data_dir 显式选中旧库所在目录。

## 升级与回滚

- 关闭所有使用目标库的 CloudMatrix 进程，确保 SQLite 已完成检查点。先备份配置和冷数据库，再替换程序；安装器不会自动执行备份。
- 回滚时使用同版本的程序与对应备份。程序会拒绝打开高于所支持版本的 schema；不要通过降版本号强行打开数据库。
- 如果备份或回滚遇到 WAL、SHM 或回滚日志文件，先核对是否仍有进程在使用数据库。不要手动删除这些文件。

## 卸载

从安装目录运行 uninstall.ps1。脚本只移除两个已知程序文件，保留数据、配置、安装清单和脚本。清除用户数据不由此脚本处理。
