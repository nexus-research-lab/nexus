# settings receipt review/reconcile evidence

本目录记录 2026-09-18 Nexus 配置 durable receipt review/reconcile 子批次。所有
提交和依赖只保留在本地 worktree，未推送，也未修改 main worktree。

## Scope

- `ReviewChange` 重新读取同一 owner/scope 的脱敏 receipt、当前快照、revision 关系和 checks。
- `ReconcileChange` 只接受当前 revision、`applied|not_applied` 和人工确认；只更新 receipt，
  不重放原始配置写入。
- round-scoped Agent 可以 review，但不能提交人工 reconcile；备注只记录是否存在，不落正文。
- `nexuscfg review`、`nexuscfg reconcile` 和 loopback broker 的 Agent 拒绝路径均已接线。

## Commands

| Evidence | Command | Result |
| --- | --- | --- |
| `configuration-review.log` | `GOWORK=off go test ./internal/service/configuration -run 'TestConfigurationReceiptReviewAndHumanReconcileDoesNotReplay' -count=1` | exit 0 |
| `configuration-review-race.log` | `GOWORK=off go test -race ./internal/service/configuration -run 'TestConfigurationReceiptReviewAndHumanReconcileDoesNotReplay' -count=1 -timeout=2m` | exit 0 |
| `target-packages.log` | `GOWORK=off go test ./internal/service/configuration ./internal/cli ./internal/app/runtime -count=1` | exit 0 |
| `architecture.log` | `GOWORK=off make check-architecture` | exit 0 |
| `desktop-gate-report.json` | `NEXUS_SANDBOX_TEST_BINARY=/private/tmp/nexus-settings-receipt-gate/nxs make check-desktop-sandbox` | host integration passed; exit 0 |

桌面 gate 使用 SDK `9d60e166` 构建的 nxs（SHA-256 `374a022e84a1dd081c2c9e2b56474dcc61dbfd4868b70f9fbeaf05de8ff49330`），
Bridge 固定为 `v0.1.34-0.20260918074231-6ea7730`，scope 是无模型请求的 host integration-only；
它不替代 macOS 原生工具链、Claude 真实会话或其他平台验收。

## Evidence boundary

本批次证明的是 durable unknown 的显式 inspect/reconcile 控制面和不重放约束。它不证明
跨进程 all-or-nothing/CAS、父目录 fsync、跨重启 revision 完整性密钥、设置页原生 UI、
Provider/网络/辅助进程隔离、完整后代清理、Claude 真实认证会话、Windows/macOS/Linux
实机或安装包发布验收；`releaseAccepted=false` 继续成立。
