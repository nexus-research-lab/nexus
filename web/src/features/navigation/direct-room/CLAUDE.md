# Direct Room 导航

- 所有 Agent 私聊入口先解析服务端 Direct Room，再生成统一 Room Conversation 路由。
- Agent 不存在时的当前选择清理与目录刷新属于导航恢复，不得回流到 API 客户端。
- 初始消息只在路由边界编码，调用方不得重复拼接查询参数。
