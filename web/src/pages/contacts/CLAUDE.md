# Contacts 页面

- 页面入口只在目录、详情和弹窗之间装配，不直接调用 Agent 或 Room API。
- `controller/` 持有联系人资源、编辑事务和删除确认状态。
- `orchestration/` 解释 `agent` 查询参数并负责 DM、单成员 Room 和删除后的路由跳转。
- 编辑器状态使用互斥联合类型，禁止同时维护 mode、open 和 agentId 三份可冲突状态。
