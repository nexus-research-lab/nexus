# Contacts 视图

- 本目录只提供联系人目录、卡片和详情视图，不读取 URL、Store 或调用 Agent/Room API。
- Agent 管理目录仅在手机与窄窗使用紧凑单列摘要卡；`md` 起恢复 comfort 大卡片，并在铺满工作面的共享管理内容区内逐级扩展到桌面三列。目录标题、说明与搜索复用共享正文 Header，不得另建全宽 Surface Header；主体动作由本领域以独立按钮承载，底部动作不得嵌套在共享卡片交互语义中。
- 联系人侧栏行与管理目录的 Agent 卡片主体必须进入同一个 `agent` 查询参数详情页；既有 Agent 只在详情页编辑，目录卡片不得另开一套编辑弹窗。
- 详情页复用 Agent Options 的可编辑字段投影、保存命令和名称校验；Header 不重复提供返回目录动作，桌面由常驻联系人导航承担定位，手机由应用级 Header 提供返回。
- 详情 Header 下缘由 `contacts-agent-detail.tsx` 在能力页标准正文 gutter 内统一绘制分隔线；身份、技能和工具沿用标准正文全宽，记忆与联络共用同一正文 gutter 和 `288px + 8px` 双栏分割轴。
- 从联系人侧栏切换 Agent 时保留当前详情栏目；只有离开详情页导致组件卸载时才恢复“身份”。
- 联系人目录已提供当前 Agent 的头像与名称，详情 Header 只承载栏目和协作动作，不重复身份；栏目顺序固定为身份、技能、记忆、工具、联络。Echo 是用户级设置，不进入 Agent 详情。
- 既有 Agent 的联系人详情采用延迟自动保存并在 Header 给出轻量状态，不保留底部保存按钮；删除是独立危险操作，桌面固定在 Header 最右侧，窄窗进入同一右上角动作菜单并继续复用页面确认链路。
- 桌面详情把聊天与发起群聊投影为同尺度的中性 ghost 工具，手机收进 `contacts-agent-detail-actions-menu.tsx`；普通协作入口不得伪装成蓝色 primary 或带外框的分段控件。
- 视图回调由页面消费者定义，保持具体且不暴露整页控制器。
- “联络”栏目由 `agent-communication-view.tsx` 直接呈现 Agent 视角的好友私聊客户端：左侧只列好友并提供搜索/添加，普通群聊继续使用“聊天”入口；右侧必须用 `WorkspaceSurfaceHeader` 与 `WorkspaceConversationTabs` 组成和聊天页同构的单行 Header，并复用 `ConversationPanelLayout`、`MessageItem` 和 `ComposerPanel`，不得复制消息气泡、输入壳、通讯录配置页或独立记录页。
- 好友首次联络没有既有 Session 时也必须显示 Composer；首条手动消息由通讯发送接口原子确保隐藏通道，并用回执 Session 接续历史。
- 好友私聊向上滚动时复用共享历史加载与前插锚定，按 `timestamp + message_id` 游标拉取更早消息，不得回退为扩大一次性 limit。
- 联络 Header 可删除双向好友关系，但不得删除隐藏 Room 和消息历史；再次添加同一好友对时恢复原通道。
- 添加好友使用 plain 选择表单；标题和提交动作不重复显示人物图标，候选列表中的头像与选中状态承担识别语义。
- 联系人总侧栏用 `CirclePlus` 表达新建 Agent，Agent 联络通讯录用 `UserRoundPlus` 表达添加好友；两者尺寸一致但不得共用图形语义。
- 侧栏“联系人”表达通讯录入口，目录页标题使用“智能体管理”表达创建和配置职责；不得再叠加 `Agents / AGENTS` 双重标题。
