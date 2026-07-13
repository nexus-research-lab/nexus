# Room 页面复合模型

- `room-page-model.ts` 分阶段构造 Room、Conversation 与 Agent 的具体投影，不执行请求或订阅。
- `use-room-page-model.ts` 只缓存基础投影并接入外部 Session 资源。
- 外部 Session 与 Room Session 在此合并，视图和根控制器不得重复解释路由身份。
