# Contacts 控制器

- 联系人集合只包含可管理的非主 Agent，编辑和删除命令不得再次判断另一套成员规则。
- 创建与更新复用 Agent Options 的字段投影和 mutation 参数。
- 创建控制器只生成 Agent Options Dialog 的关闭/创建判别状态；既有 Agent 的更新由详情页内联编辑器处理。
- 删除命令返回具体 Agent ID，是否离开当前路由由页面协调器决定。
- `use-agent-communication.ts` 绑定详情 Agent id，只加载好友、打开联系人通道、切换私信 Session、轮询当前消息并以选中 Agent 身份发送；旧作用域响应不得写回新 Agent 或新联系人。
