# CloudMatrix 性能与基准测试套件

本目录存放容量等级与长稳实验用例，依据 `docs/03-validation-plan.md` 设计：

- `CM-1`：单实例基线负载与投屏性能
- `CM-5`：5 实例多任务与并发启停
- `CM-10` / `CM-20`：高并发容量等级测试（需在独立生产工作站/高性能服务器上执行）
- 压测执行入口：`scripts/benchmark.ps1`
