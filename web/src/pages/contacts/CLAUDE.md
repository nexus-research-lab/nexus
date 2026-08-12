# Contacts 页面

- 页面入口只在目录、详情和创建弹窗之间装配，不直接调用 Agent 或 Room API；既有 Agent 的管理入口统一落到详情页。
- `contacts-page-model.ts` 统一投影加载/目录/详情联合状态和删除确认文案，不复制编辑器初值。
- `controller/` 持有 Agent 目录、当前详情 Agent 的好友、私信 Session、消息轮询与发送事务，以及编辑和删除确认状态；普通群聊不进入联络资源。
- `orchestration/` 解释 `agent` 查询参数并负责 DM、单成员 Room 和删除后的路由跳转。
- 创建编辑器状态使用互斥联合类型，不得同时维护 mode 与 open 镜像；既有 Agent 的编辑状态由详情路由承载，不再保留第二套编辑弹窗快照。
