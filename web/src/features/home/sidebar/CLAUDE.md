# sidebar/ - Home 侧栏

- `sidebar-directory.ts` 只提供共享 Home 目录；聊天和联系人入口都不得在侧栏订阅 Agent runtime。
- `../room-activity-resource.ts` 以 Room ID 维护瞬时执行集合；聊天行只判断自身 Room 是否 active，DM 与群组不分叉。
- `sidebar-conversation-model.ts` 只投影 Room/DM 目录项；未读状态由 `sidebar-unread-model.ts` 统一聚合。
- `use-chat-sidebar-controller.ts` 负责聊天列表导航、Room 创建和删除事务，视图不得直接调用 API 或 Store 命令。
- `chat-sidebar-panel.tsx` 与 `contacts-sidebar-panel.tsx` 是两个独立入口，不再通过聚合文件互相耦合。
- `sidebar-list-rows.tsx` 以头像、元信息、状态和摘要子视图渲染目录行；手机聊天目录使用 80px 行高和 40px 头像，ContactRow 保持 72px 密度且只显示静态 Agent 目录信息，不推导运行态。
- 聊天与联系人目录的当前行复用主题活动底面、细边界和软阴影，浅色主题采用低饱和暖象牙灰，非当前行只降低文字明度；不得恢复左侧品牌色窄活动标记。
- 普通聊天目录点击进入 Room 根路由，由页面恢复该 Room 最后激活的 Conversation；存在明确未读 Conversation 时仍直接进入未读目标。
