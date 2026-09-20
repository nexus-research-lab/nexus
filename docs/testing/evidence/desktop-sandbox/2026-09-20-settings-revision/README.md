# 配置 revision 跨重启恢复证据（2026-09-20）

Nexus 实现提交：`219f4f53dc194db8fa67a2aeb946f4eb94c984eb`。SDK 与 Bridge 沿用原精确本地提交，未推送。

旧实现的数据库重开反例先失败；新实现通过配置/存储目标包、定向 race、migration/CLI/app runtime/server、vet、架构与真实 nxs host gate。完整命令、最终退出码、版本和限制见 [report.json](report.json)。[desktop-gate-report.json](desktop-gate-report.json) 记录原 gate 的基线提交与当时 dirty 范围；测试源码随后固化为上述提交，文档在测试后补齐。

原始日志使用 gzip 保存，包含预期失败以及三个具名 host gate 组的 Go JSON 事件。`manifest.json` 校验本目录各文件；`report.json` 另记录实现提交的变更文件哈希。固定二进制可按 gate 报告的 SHA-256 核对，再执行报告内桌面门禁。

证据只证明 Nexus 配置控制面的持久版本核对。它不证明 SDK 多文件掉电事务、跨进程完整 reconcile 原子性、PostgreSQL 原生、凭据/辅助进程/网络、Claude 命令沙箱或跨平台安装验收。`releaseAccepted=false`。
