# sidebar/ - Home 侧栏

- `sidebar-directory.ts` 只提供共享 Home 目录；聊天和联系人入口都不得在侧栏订阅 Agent runtime。
- `../room-activity-resource.ts` 在每个 Room ID 内按精确 Conversation/Session source 隔离瞬时执行集合，再为聊天行取并集；DM 与群组不分叉，另一个空会话不得清掉仍运行的会话，待确认优先于工作中。
- `sidebar-conversation-model.ts` 只投影真实 Room/DM 目录项；主智能体 DM 固定置顶且不可删除，其他条目仍按最近活动排序，活动时间按当前界面语言格式化；未读状态由 `sidebar-unread-model.ts` 统一聚合。
- 聊天行摘要只复用 bootstrap 已有的会话标题，目录首屏不得为预览读取完整消息历史；目录首次失败必须与真实空目录分开展示并提供重试，后续刷新失败继续展示最后成功目录并使用非阻塞提示。
- `use-chat-sidebar-controller.ts` 负责聊天列表导航、Room 创建和删除事务，视图不得直接调用 API 或 Store 命令。
- `chat-sidebar-panel.tsx` 与 `contacts-sidebar-panel.tsx` 是两个独立入口，不再通过聚合文件互相耦合。
- 联系人搜索右侧的 `UserPlus` 是创建智能体的直接入口，必须以 `view=create` 路由意图打开共享 Agent 编辑器；联系人空态中的管理动作仍只进入目录。
- `sidebar-list-rows.tsx` 以头像、元信息、状态和摘要子视图渲染目录行；桌面与手机的聊天、联系人身份锚点统一使用 40px 头像，手机聊天目录使用 80px 行高，ContactRow 保持 72px 密度且只显示静态 Agent 目录信息，不推导运行态。Room 工作态使用低饱和蓝灰 `running` 语义，待确认使用 warning 徽标；两者都保留头像活动外圈，不复用品牌 CTA 或完成态绿色。
- 聊天、联系人和能力目录的当前行统一使用共享侧栏中性浅灰底面，不显示独立边框或浮起阴影；非当前行保持透明且没有框线，只降低文字明度，不得恢复左侧品牌色窄活动标记。
- 桌面导航轨、目录栏和主画布之间不得使用贯穿高度的硬分割线；导航轨使用最深的主题薄材质，目录栏使用中间材质，画布保留环境底面，交界只由宽而低对比的主题阴影表达。浅色、深色和雨夜背景必须复用同一层级语义，不得在组件中写死某一主题颜色。
- 普通聊天目录点击进入 Room 根路由，由页面恢复该 Room 最后激活的 Conversation；存在明确未读 Conversation 时仍直接进入未读目标。
- Launcher bootstrap 幂等保证主智能体默认 DM 存在，因此聊天空态不再承担“先创建联系人”的首启分支。
