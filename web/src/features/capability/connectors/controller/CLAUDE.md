# Connector Controller

- catalog、detail、commands 分别持有列表、详情和命令状态。
- `use-connector-command.ts` 是唯一命令互斥入口，动作使用判别联合表达。
- 列表和详情请求使用递增请求号拒绝过期响应。
- 命令完成后的列表/详情刷新由组合控制器统一编排。
