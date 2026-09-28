# 显式 scratch 与策略恢复

本批基于 Nexus `0ec0df576`，最终源码版本为包含此文件的提交。固定 Bridge `c994b197e010`。本批没有 Windows 验证或真实 Provider 请求。

## 行为与证据

- macOS confinedfs 通过固定 parent 句柄、不覆盖目标的原子移动隔离目录；移动后再核验原 inode，目录同步后才推进阶段。目标重名、原目录替换及路径穿越拒绝，操作不跟随被替换的 parent 发布路径。
- SQLite 迁移 148 保存 prepared/quarantined/deleting/complete。进程未收口、资源身份不符或同会话存在活跃启动时不能领取清理。pending 同时阻断 Manager factory 和数据库 PrepareProcess；降级不允许丢弃清理记录。
- 六个提交故障场景覆盖 quarantined/deleting/complete 的提交前失败及提交后响应丢失。重试使用原记录，完成后不删除后续同名目录。源路径缺失不猜成功；quarantined 阶段目标缺失或被替换保留阻断；只有 deleting 阶段能对账删除后的缺失。
- ReconcileSandboxPolicy 只收口明确绑定原进程的策略；未完成 scratch 时拒绝，无关联历史 unknown 保持原样，原 unknown reason 保留。没有更新业务结果或重放工具。

## 原生宿主退出链路

`TestRecoveryNativeHostExitWithScratchAndPolicies` 在独立测试宿主中取得真实实例锁、创建 lease，启动真实 helper/launchd 下的受控 shell/sleep，然后 os.Exit 跳过所有清理。父测试等待其确实退出，取得同一状态根锁，并确认原任务仍显示 running。

新宿主先拒绝过早 scratch/策略恢复，再恢复原进程得到 `coalition_reaped`，隔离并删除原 scratch，收口两份绑定回执，最后验证新启动栅栏解除。进程、实例锁、数据库与文件动作是真实执行；策略回执是测试注入，任务不是 nxs/Claude 模型调用，不能作为 App 界面或真实 Provider 验收。

原生测试首次因夹具未创建 canonical runtime 父目录而失败（尚未启动任务）；补齐夹具初始化后通过。

## 复验命令

```sh
GOWORK=off go test -race -count=1 ./internal/infra/confinedfs -run '^TestQuarantine'
GOWORK=off go test -race -count=1 -v ./internal/runtime -run '^TestScratchRecovery'
GOWORK=off go test -race -count=1 -v ./internal/runtime ./internal/storage/sandbox -run '^TestScratchAndPolicyRecovery|^TestScratchRecoveryDoesNotGuess|^TestPolicyRecovery'
GOWORK=off go test -race -count=1 -v ./internal/storage/sandbox -run '^TestScratchRecoveryLedger'
NEXUS_SUPERVISION_TEST_HELPER=<signed-helper> GOWORK=off go test -race -count=1 -v -timeout=120s ./internal/runtime -run '^TestRecoveryNativeHostExitWithScratchAndPolicies$'
GOWORK=off go test -race -count=1 ./internal/runtime ./internal/storage/sandbox ./internal/infra/confinedfs ./internal/protocol
GOWORK=off make check-architecture
```

helper SHA256: `7e21e16effd3c00c0d2805d8c09cc850470cdf6b11d37eb5677c5f1dc31735a8`。

## 交付边界

这是显式调用的恢复能力。App 默认启用、任务不可访问的宿主根配置、正常退出后的完整清理事实、重启扫描已 reaped 进程的未完成资源/策略步骤仍需接线与验收。进程 pending 扫描当前不会自动覆盖所有后续阶段，不能仅凭空扫描声称完整恢复。历史无 proof/无关联记录继续保守保留。PostgreSQL 迁移仅 SQL 审查，运行证据来自本机 macOS arm64 + SQLite。macOS 14.0/Intel、真实 App 模型全流程与正式发布验收仍未完成。
