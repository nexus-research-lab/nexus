# Connector Controller

- catalog、detail、commands 分别持有列表、详情和命令状态。
- `use-connector-command.ts` 是唯一命令互斥入口，动作使用判别联合表达。
- 列表和详情请求使用递增请求号拒绝过期响应。
- 命令完成后的列表/详情刷新由组合控制器统一编排。

- 普通 catalog/detail 读取失败保留已知快照；访问拒绝立即丢弃对应快照，后续重读不能在成功前恢复旧数据。

- 对账已按 effect 分离：committed 读取成功才清锁；unknown 读取成功仅提供显式新操作；accepted 保持只读对账。OAuth 成功事件可以确认本次授权完成，普通目录成功不能替代该结果。成功、授权与失败反馈统一由双语目录提供，不改变协议值和恢复阶段。
