# Windows 本机组件与恢复基线

2026-09-28，Windows 11 Home 10.0.26200 amd64、Go 1.26.8、Node 22.23.2。
主验证进程为非提升权限用户，所有 Go 操作使用 `GOWORK=off`。结果来自本机执行，
未等待 GitHub Actions。此报告为开发工作树证据，`releaseAccepted=false`。

固定依赖：

- SDK `2148b4b1833b2324a4f41f6e42235aa9a73424e5`，干净 checkout。
- Bridge `v0.1.34-0.20260927154842-c018b4973dc3`，远端 Go 模块，无 replace。
- Bridge module checksum `h1:7pow+GjKN+I/c517IR/atBuDT9B5sQIvvio2rFN8mp8=`。

| 本机门禁 | 必测通过数 | 证据边界 |
| --- | ---: | --- |
| 进程身份 | 2 | 当前进程创建时间、保守未知处理 |
| 宿主资源 | 6 | 强制退出后的显式 reconcile、unknown 保留、资源复制、Windows Claude 拒绝、原子替换并发读、junction 越界拒绝 |
| Bridge 生命周期 | 8 | Job 绑定前无入口执行、立即派生的后代回收、继承管道时取消/自然退出、宿主被终止、预检参数/环境和能力缺失拒绝 |
| SDK Windows 组件 | 25 | 原有 token/Job/private desktop/pipe/runner/fail-closed 组件 |
| SDK settings | 12 | 并发与多文档写入、回滚、硬链接隔离、日志无明文、长路径、现存日志保护、跨根全旧/部分/全新提交后强制终止恢复 |

53 个指定测试及指定子测试均实际通过，未将 skip 或仅 package 成功计入。
同时通过 installer contract 检查，并编译 `internal/runtime`、`clientopts`、
`confinedfs` 和 `nexus-server`、`nexusctl`、`nexuscfg` 的 Windows amd64/arm64 产物。
arm64 只有编译证据。

额外定向回归：SDK config/permission 包通过；Bridge 全包测试通过（其他需外部
runtime/Unix 工具的用例仍有 skip）；Nexus runtime、clientopts、confinedfs、receipt
storage 各目标包通过。SDK 的强制终止测试确实杀死仍持锁的写入者，验证重开锁和
日志恢复，不是手写文件状态模拟，也不模拟操作系统崩溃或掉电。

可复现入口：

```powershell
$env:NEXUS_SANDBOX_SDK_SOURCE = (Resolve-Path '..\nexus-agent-sdk-go').Path
$env:NEXUS_SANDBOX_REPORT_DIRECTORY = Join-Path $env:TEMP 'nexus-windows-evidence'
node scripts/desktop/check-windows-sandbox.mjs --native
```

入口记录 Nexus HEAD/工作树状态、SDK SHA、Bridge version/checksum、每条命令及原始
stdout/stderr，任何必测项失败、skip 或缺失都拒绝通过。

尚未闭合：专用账号下同一候选的完整 PowerShell/后代与 host/runner 控制拒绝矩阵、
原生命令与文件/搜索/媒体/Notebook/Skill/context/project/settings/policy 执行接线、
文件 DACL/网络/凭据边界、账号设置与修复升级、真实安装包、Windows arm64 实机。
Bridge 创建进程与绑定 Job 仍是两步，宿主在绑定前崩溃可能留下尚未执行入口的挂起
进程；不声称原子创建回执。Windows settings 验证进程退出恢复，不声称目录项的掉电
持久性。完整 Windows nxs 沙箱能力继续关闭，不能用上述组件通过取代 P3/P4 验收。
