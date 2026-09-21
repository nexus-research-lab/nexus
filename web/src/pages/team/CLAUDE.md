# Team 页面

- 私聊与群聊共用真人消息方向：只有消息作者 ID 精确匹配当前 Control 用户 ID 才靠右；其他真人靠左，身份目录缺失不改变方向。

- 等待领取的投递向原消息作者提供取消入口；Relay 校验作者或 Agent 所有者及 pending 状态，已领取任务不能作为取消等待处理。失败保留重试并重新读取权威状态。
- Agent 显式文件输出复用附件展示与 saveTeamFile；纯文件回复也保留 Agent 身份和下载入口，失败可见，不构造私人工作区路径。

- 在线资源没有固定刷新间隔。目录、邀请、绑定、文件及 Room 详情通过 owner-scoped directory WS 失效提示单飞补读；本机任务仅由原生 Room 事件唤醒。首次、重连、焦点/网络恢复和手动恢复仍读取权威快照，提示不包含群内容。

- 本机执行绑定准备完成即挂载原生 Room 订阅，不等待 Agent 回复或打开 Thread。任务关联读取禁用定时器，WS 连接/执行变化驱动补读；同会话只订阅一次，round 与本机 Agent 精确匹配，native 终态优先于尚未更新的任务运行态。

- 回复来源徽标同时显示成员身份与被回应消息的截断摘要，Tooltip/ARIA 保留完整正文；圆角复用 radius-control-sm，不另建胶囊风格。

- 本机任务 cancelled 表示确认中断，不等于 failed；Thread 优先采用原生 cancelled 事实。存在精确 delivery/source_message 绑定时，头部复用本地 Room 的 MessageReplyChip 显示来源姓名和头像；不生成虚假 handoff，不按发布时间猜测关联，不改变 Relay 消息顺序。

- Agent 消息的 content.execution 只包含模型和白名单终态统计，适配到 MessageItem 原生底栏；缺失数据不补造。私人记忆引用不共享，在线消息不冒充本机会话提供分叉动作。

- 在线 Agent 回复直接适配到 Room 共用的 `MessageItem` / `room_result`，不手写身份头和正文；只表示完整公开消息，不伪造远程执行流或权限。

- 顶部辅助栏目复用 `buildRoomHeaderTabs`，工作图/子智能体/工作区/简介与 Thread 互斥。成员绑定不依赖任务历史，模型与权限设置只编辑本人本机 Session。群文件经 Composer 上传后以不可变引用进入消息；消息附件复用 MessageUserSection 与作用域明确的下载回调，不向在线草稿注入本地路径。

- 在线输入直接使用 `ComposerPanel` 的草稿、输入法和内联 @；不再保留独立 textarea/目标下拉框。本机任务在页面 Thread 打开，不进入聊天目录；任务元数据刷新后使用当前绑定。

- `team-page.tsx` 只允许已登录 Control 远程账户按 `room_id` 适配 Relay 真人消息模型；本地免登录用户返回本地聊天首页。Header、消息阅读轨道、本人消息和 Composer 外观复用 Room 的共享 UI 原语，加载、同步和发送仍交给 `features/team/use-team-room.ts`。
- 显示真人消息与 Agent 完整 assistant/final 回复，不渲染远程 Agent 流；本人入群 Agent 随 `/team-node/room` 同步自动登记执行，Header 不再提供本机授权入口。
- Header 的成员入口读取 Relay Room 管理快照；真人群主和管理员可以邀请、移除，真人群主还可以改角色和移交治理权；Agent 与真人分区显示，每名 active 真人可添加自己的 Agent。

- Enter 发送先排除输入法组合事件（含 keyCode 229）；Shift+Enter 保留换行。空内容、加载前或发送中不得受理提交；失败保留草稿。加载与失败具备 status/alert 语义，加载失败不声明空会话。

- 所有真人消息复用 `MessageUserSection` 的阅读布局、气泡、时间、折叠和复制；其他真人通过可选 author 显示共享头像与姓名，本人不重复身份。不再手写左侧头像 gutter 和另一套正文排版。

- 滚动复用 conversation 的 useFollowScroll 与 ScrollToLatestButton：底部跟随，上滚后保持阅读，显式回到底部恢复跟随。Team 不维护独立滚动阈值、动画或强制 scrollIntoView；会话 identity 为实际 conversation.id。可聚焦 region 支持键盘滚动，局部 lint 例外仅用于该滚动区域。

- 首次加载失败提供公共“重试”按钮，仅调用 retryLoad；加载中禁用且标记 busy。不得用重试入口重发聊天消息。

- 已有消息在读取刷新期间继续可见，列表标记 busy；只有没有消息的首次加载才显示整块加载提示。

- 页面实例按 owner generation、用户与路由 room_id 隔离，草稿和未完成发送不能越过会话切换。
- 成员弹窗的新快照回传 `useTeamRoom.updateDetails`，Agent 目录随成员版本重读；选中的目标失效后仍保留可移除 chip，不降级成普通消息。未确认发送冻结输入与目标选择，仅保留原请求重试。
- Composer 恢复发件箱的原正文，不能用当前空草稿覆盖未知命令；失去群访问权后禁用输入。Header 使用远端 Room 当前名称和头像，解散/退出完成后重新核对目录。

- 顶栏复用 Room 的 WorkspaceConversationTabs 与 GroupMemberAvatarStack：在线唯一会话不提供关闭、新建或固定；成员摘要仅计 active 真人和 Agent，目录只补名称与头像。

- 真人 DM 通过 `room.direct_user_id` 解析对方姓名头像；消息沿用同一 Feed/Composer。`room_invitation` 是 Relay 生成的持久卡片，操作必须匹配当前待处理邀请和邀请时间；旧邀请没有消息卡片时在对应私聊补显示待办，不能根据卡片直接推断授权。

- 在线消息只将 Relay 显式 mentions 投影成 Room mention chip，正文匹配使用 Unicode rune 偏移，不从普通文本推断新的执行目标；远端身份不跳转本机 Agent 联系人。已有回复的本机任务只在 Agent 回复头保留 Thread；未回复任务在触发消息后复用 MessageItem 展示 Agent 身份、任务活动/状态和 Thread，不再裸放按钮。
- 在线 Composer 从 /team/commands 获取原生产品提示命令目录（plan、browser、visualize、workgraph），显式 @ 后按 Relay 目标成员栅栏分发，接收节点复用 Room 的运行时命令展开；不发布宿主管理命令或其他人的私人 Skill 目录。

## 在线与本地 Room 对照（2026-09-17）

| 展示面 | 统一入口与状态 |
| --- | --- |
| 真人消息 | `MessageUserSection`；其他真人只增加身份信息，不重建气泡和复制动作 |
| Agent 回复与未回复占位 | `MessageItem` / `room_result`；未回复时仍显示身份与任务状态 |
| Agent 操作条 | `RoomAgentExecutionActions` + `ThreadActionButton`，不把 Thread 放在真人名字旁 |
| Thread 状态 | 原生执行状态决定活动态；公开 final/任务终态补读历史，历史加载不等于正在思考 |
| Thread/辅助侧栏 | 原生尺寸约束、鼠标与键盘 resize；窄屏使用 Room overlay，隐藏重复 Thread 副标题 |
| 顶部栏目 | 原生栏目模型与成员头像栈；在线只有一个共享会话，不提供假的新建/关闭会话按钮 |
| 输入与 @ | `ComposerPanel`、共享 mention picker/chip；候选浮层在整个输入壳上方 |
| 群设置 | Room 共同表单原语；人员/Agent 分区，当前远程账户补齐本人头像 |
| 阅读与恢复 | 原生 follow-scroll；刷新保留消息，错误可重试，发送失败保留草稿 |
| 消息附件 | 上传复用群文件 API，冻结引用到发件箱；支持无正文附件、消息下载和节点原生 Room 输入，上传失败不降级为纯文本 |
| Slash 命令 | 共用补全 UI 和服务端产品命令定义，显式 @ 目标后进入原生 Room 展开；不远程调用宿主管理入口 |
| 执行操作 | 原生 Room 订阅提供精确 agent_round 停止与 stopping 状态；主 Composer 复用权限/问答队列，仅接受当前群本人绑定。桌面 Thread 不重复审批入口，窄屏覆盖层保留入口；绑定卸载即清除动作 |

只共享公开 assistant/final 消息；其他成员的本机流、权限请求和私人工作区不由 UI 对齐而放开。

- Composer 上方的绑定读取、任务状态、消息加载/同步错误统一复用 `UiInlineNotice`，共用阅读宽度、字号、间距和恢复按钮；每项仍执行自己的读取重试，自动同步与未确认发送不借用加载重试动作。
# 共享投递进度

- 远程 pending/leased 等待行复用本地 `MessageActivityStatus` 的图标、LoadingOrb、稳定高度与共享 Room 左对齐规则，保留真实投递文案；终态停止动效，不将远程领取状态伪装成本机思考流。

- 远程 Agent 状态来自 RoomDetails.deliveries；本机执行才显示 Thread。中间 assistant 消息不清除远程状态，final 或 completed 才清除。来源消息由 delivery.message_id 关联，不按姓名或顺序猜测。

- 回复来源为本人时，头像从当前登录身份补齐；可邀请成员目录排除了本人，不可作为本人头像的唯一来源。Agent 来源不会继承其拥有者的真人头像。
