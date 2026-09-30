# 宿主持久进程登记与启动栅栏

2026-09-28，Nexus `2f24c12e5565154536fe2a12ba974c80e859459b`。`productionBootstrapIntegrated=false`，`releaseAccepted=false`。

## 已实现并验证

- SQLite 迁移新增宿主数据库进程启动事实，不存任务参数、环境或 Provider 凭据。记录绑定原有 owner/session/generation，并增加唯一 launch ID、job label、helper digest、boot/UID 和可选 lease。它不替代连接后的策略回执，也不从用户可写 scratch 标记恢复可信进程身份。
- 一条 owner/session 只允许一个未收口启动。注册原集合后，一次性领取才能进入 released；领取不提供幂等成功重放。真实 SQLite 的 8 个并发领取者恰好一个成功，数据库重开后不能重新领取。
- 旧代次、跨 owner、变更原意图、已 abort 后的迟到登记、已回收后的迟到放行、错误 coalition/boot/UID 和 root_exit 伪证明均拒绝；损坏的行键与 JSON 身份绑定读取失败。
- runtime 在调用 factory 前读取数据库进程事实；prepared/registered/released 即使没有策略回执也阻断 nxs/Claude 重建。合法终态和原策略回执共用代次下界。读取错误、身份不符和缺失回收事实不放行。

[仓储竞态检查](registry-race.txt)、[runtime 包全部测试](runtime-package.txt)、[启动目标测试](startup-targeted.txt)通过。目标包 vet 与架构依赖检查通过。没有运行全仓 Go 测试、Windows 验证或 PostgreSQL 实例验收；PostgreSQL 迁移只做代码审查，不将 SQLite 结果当作其运行证据。

```sh
GOWORK=off go vet ./internal/runtime ./internal/storage/sandbox
GOWORK=off go test -race -count=1 ./internal/storage/sandbox
GOWORK=off go test -count=1 ./internal/runtime
GOWORK=off make check-architecture
```

## 尚未接通

生产 launcher 尚未调用 Prepare/Register/ClaimRelease/Reap 写入链；当前仅落地数据库合同与产品读栅栏。实际 bootstrap 启动、helper/job 核验、一次放行、退出观察、撤销与回收、policy/lease unknown 对账仍待连接。aborted 只证明未领取执行放行，不证明可信 helper 已退出，仍需清理其 job。仓储验证回收事实的 exact binding，不能独立证明内核观察真实。

SQLite 使用独立测试数据库并跑版本迁移，不是完整历史 App 数据、升级回退或签名包验收。原 Bridge pin 保持不变；本次没有启动模型或用户 App，也没有操作已有 unknown 记录。
