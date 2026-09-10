# Internal 模块边界与治理验收

本方案维护模块化单体的依赖与数据归属。状态：已实施并验收（2026-09-09）；下表是本轮完整交付范围。

## 目标依赖

- HTTP、CLI、MCP 负责协议解析和入口鉴权，app 负责显式装配。
- 业务服务持有状态转换与跨域流程；仓储持有本领域 SQL 和事务操作。
- runtime 根包管理子进程、会话与轮次，不依赖 app 或业务服务。
- protocol 保持底层合同；领域内部事实放在领域包，不向 protocol 堆入私有实现。
- 外部 Relay 合同独立于客户端实现；存储和同步服务消费同一合同。

## 实施顺序与验收

| 项目 | 实现归属 | 必须保持的行为 | 验收 |
| --- | --- | --- | --- |
| Team 同步 | `internal/relay` 合同，`service/relay` 客户端，`service/team` 同步流程 | 远端消息已提交后本地投影失败仍返回成功；目录、建群、快照和增量投影失败不能确认成功；身份与令牌仍只来自可信入口 | handler 契约、服务失败路径、仓储连续游标与幂等测试 |
| Execution 命令观察 | orchestration 操作事实，runtimehook 回执转换 | 请求身份、责任归属、拒绝状态、重放幂等与节点关联不变 | 回执转换与 Runtime Graph 回归 |
| Session 删除 | deletion 协调，automation/orchestration 仓储事务内清理 | Goal/Task 原有准备顺序不变；路由与执行数据继续同事务提交；owner 隔离、历史审计保留 | 中途失败回滚与 owner 隔离测试 |
| 历史投影 | message 负责结果归属与合并，storage/workspace 负责存取与查询 | 保留公共分页入口、物理 Agent round 配对、旧记录兼容与未匹配结果合成；不迁移历史格式或缓存模型 | 混合 Agent 结果、重放与历史分页回归 |
| 架构门禁 | Go 导入检查与现有 Go 检查流程 | 检查生产导入，允许集成测试装配；无动态例外清单 | 实际依赖图与禁止边界的负例 |

## 事务与适配约束

删除协调器是既有跨表事务的唯一提交者。领域仓储接受同一个 `sql.Tx`，不自行提交，也不把现有事务拆成多个服务调用。Goal/Task 的前置清理仍是原有可重试流程，不声称跨文件、runtime 与 SQL 已具有全局原子性。

Team 同步服务不新增隐式重发或后台消费者。投影恢复继续使用既有 Snapshot/Difference 与原游标，不能把结果未知当作未提交重试。

历史层本轮复用已有 `readHistoryPageWithIndex`，不增加另一套查询门面。优先抽离已稳定的结果投影；文件安全、分页索引、源解析和删除栅栏继续同包维护，避免为目录划分暴露内部模型。

## 验证记录

- 目标包与调用方测试通过：app、CLI、Team handler/service、Relay、deletion、orchestration/runtimehook、DM、Room、session、automation、相关仓储、message 与 nexus-server。
- 上述受影响包的 `go vet` 通过。
- Team 投影失败、Session 删除事务回滚和 owner 隔离、历史结果归属、运行回执转换及共享 Observer 的关键测试通过 `go test -race`。
- `go test -run '^$' ./internal/... ./cmd/...` 通过，验证全部内部包与命令入口编译；此项不代表运行全部测试。
- 架构门禁及其允许/禁止依赖测试通过；门禁按当前 Go 构建环境检查生产导入。
- Go 格式、检查脚本语法和 `git diff --check` 通过。
