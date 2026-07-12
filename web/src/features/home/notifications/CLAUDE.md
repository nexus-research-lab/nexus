# 聊天完成通知域

本目录负责将 Room WebSocket 完成事件投影为站内未读状态和浏览器系统通知。

## 职责边界

- `chat-notification-target.ts` 统一路由、Room、Conversation 与 Session 的通知目标身份。
- `chat-notification-directory.ts` 只建立共享目录索引并提供目标查询。
- `chat-notification-model.ts` 只做完成事件判定、目标和通知内容纯投影。
- `browser-notification.ts` 封装浏览器可见性、权限和系统通知副作用。
- `use-chat-notification-socket.ts` 只处理 Room 协议订阅、序列游标和事件分类。
- `use-chat-completion-notifications.ts` 编排当前页面、未读 Store 与通知策略。

## 不变量

- 通知与侧栏必须消费 `home-directory-resource.ts` 的同一目录快照，不得各自加载 bootstrap。
- Room 订阅只由排序后的 Room ID 内容键驱动，目录对象换引用不得触发重订阅。
- WebSocket 重放依靠消息 ID 在 Store 中去重；活动窗口内的当前目标只清除未读，不弹系统通知。
- 浏览器权限失败不得影响站内未读记录。
