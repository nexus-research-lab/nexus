# 聊天完成通知域

本目录负责将 Room WebSocket 完成事件投影为站内未读状态和浏览器系统通知。

## 职责边界

- `chat-notification-target.ts` 通过有序身份规则和路径解析表统一路由、Room、Conversation 与 Session 的通知目标。
- `chat-notification-directory.ts` 只建立共享目录索引并提供目标查询。
- `chat-notification-model.ts` 只做完成事件判定、目标和通知内容纯投影。
- `browser-notification.ts` 封装浏览器可见性、权限和系统通知副作用。
- `use-chat-notification-socket.ts` 处理 Room/DM 事件订阅、序列游标和事件分类，并把执行与人工交互生命周期写入 `../room-activity-resource.ts`。
- `use-chat-completion-notifications.ts` 编排当前页面、未读 Store 与通知策略；精确 Conversation 在活动窗口中时直接确认目标，未读顺序只服务侧栏路由，不得进入 Feed 滚动层。

## 不变量

- 通知与侧栏必须消费 `home-directory-resource.ts` 的同一目录快照，不得各自加载 bootstrap。
- Room 订阅只由排序后的 Room ID 内容键驱动，目录对象换引用不得触发重订阅。
- WebSocket 重放依靠消息 ID 在 Store 中去重；活动窗口内的当前目标只清除未读，不弹系统通知。
- WebSocket 重连成功必须令共享聊天目录失效一次，补偿断线期间可能丢失的全局目录事件；不得恢复固定间隔的全量目录轮询。
- 通知目标优先级固定为 Room Conversation、Room、Session；Session 活动目标不得回退匹配同 Room 的其他通知。
- 浏览器权限失败不得影响站内未读记录。
- 聊天行仍只按 `roomId` 输出状态，但执行态必须在容器内部按精确 Conversation/Session source 隔离后取并集；DM `session_status` 与 Room 全局 `active_sources` 负责重连恢复，任何空快照或终态只能清理自己的 source。root round 终态负责清理该 source 的执行槽，单个 Agent slot 终态不得熄灭仍在执行的其他 slot，权限 resolved 只清理对应请求。
