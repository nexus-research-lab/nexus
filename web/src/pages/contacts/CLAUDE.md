# Contacts 页面

- 页面入口只在目录、详情和弹窗之间装配，不直接调用 Agent 或 Room API。
- `contacts-page-model.ts` 统一投影加载/目录/详情联合状态和删除确认文案，不复制编辑器初值。
- `controller/` 持有联系人资源、编辑事务和删除确认状态。
- `orchestration/` 解释 `agent` 查询参数并负责 DM、单成员 Room 和删除后的路由跳转。
- 编辑器状态使用互斥联合类型，创建保存空状态，编辑保存打开时的 Agent 快照，不得同时维护 mode、open 和 agentId 镜像。
