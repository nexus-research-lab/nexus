# sidebar/ - Home 侧栏

- `sidebar-directory.ts` 只提供共享 Home 目录；聊天和联系人入口都不得在侧栏订阅 Agent runtime。
- `../room-activity-resource.ts` 以 Room ID 维护瞬时执行集合；聊天行只判断自身 Room 是否 active，DM 与群组不分叉。
- `sidebar-conversation-model.ts` 只投影 Room/DM 目录项；未读状态由 `sidebar-unread-model.ts` 统一聚合。
- `use-chat-sidebar-controller.ts` 负责聊天列表导航、Room 创建和删除事务，视图不得直接调用 API 或 Store 命令。
- `chat-sidebar-panel.tsx` 与 `contacts-sidebar-panel.tsx` 是两个独立入口，不再通过聚合文件互相耦合。
- 联系人搜索右侧的 `UserPlus` 是创建智能体的直接入口，必须以 `view=create` 路由意图打开共享 Agent 编辑器；联系人空态中的管理动作仍只进入目录。
- `sidebar-list-rows.tsx` 以头像、元信息、状态和摘要子视图渲染目录行；桌面与手机的聊天、联系人身份锚点统一使用 40px 头像，手机聊天目录使用 80px 行高，ContactRow 保持 72px 密度且只显示静态 Agent 目录信息，不推导运行态。Room 的瞬时执行态使用独立低饱和蓝灰 `running` 语义同步徽标与头像外圈，不复用品牌 CTA 或完成态绿色。
- 聊天、联系人和能力目录的当前行统一使用共享侧栏中性浅灰底面，不显示独立边框或浮起阴影；非当前行保持透明且没有框线，只降低文字明度，不得恢复左侧品牌色窄活动标记。
- 桌面导航轨、目录栏和主画布之间不得使用贯穿高度的硬分割线；导航轨使用最深的主题薄材质，目录栏使用中间材质，画布保留环境底面，交界只由宽而低对比的主题阴影表达。浅色、深色和雨夜背景必须复用同一层级语义，不得在组件中写死某一主题颜色。
- 普通聊天目录点击进入 Room 根路由，由页面恢复该 Room 最后激活的 Conversation；存在明确未读 Conversation 时仍直接进入未读目标。
