# Contacts 控制器

- 联系人集合只包含可管理的非主 Agent，编辑和删除命令不得再次判断另一套成员规则。
- 创建与更新复用 Agent Options 的字段投影和 mutation 参数。
- 创建控制器只生成 Agent Options Dialog 的关闭/创建判别状态；既有 Agent 的更新由详情页内联编辑器处理。
- 删除命令返回具体 Agent ID，是否离开当前路由由页面协调器决定。
- `use-agent-communication.ts` 绑定详情 Agent id，只加载好友、打开或恢复已有联系人通道、切换私信 Session、按稳定消息游标前插历史、订阅当前隐藏 Room 并以选中 Agent 身份发送；断线时用低频轮询兜底，无通道时保留 Composer，由首条发送原子创建 Room，删除只移除好友关系，旧作用域响应不得写回新 Agent 或新联系人。
