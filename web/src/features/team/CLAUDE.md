# Team

- 常驻 Room 目录按登录作用域及成员版本批量准备本人 Agent，不依赖打开群；失败仅退避重试原准备操作，不增加固定任务轮询。
- 未知执行的核验按钮只请求后端核对精确 round 与 Relay 回执；停止请求成功不等于停止完成，不直接解锁或重跑。

- 原生会话绑定必须同时携带本机 `local_agent_id`、Room 和 Conversation；观察器、Thread、辅助面板均不得用 Control Agent ID 替代本机身份，或省略命令目录所需的 Agent 成员校验。

- 消息附件复用共享文件上传与有界下载；发送前把 id/name/size/sha256 冻结到原有持久发件箱，未知结果重放原引用，支持附件单独发送。UI 的 relayRoom 作用域不含本机路径，接收节点才物化为原生 Room 附件。消息下载与工作区共用 saveTeamFile。

- 执行观察器直接提供原生 Room 停止、stopping 与权限响应能力，按 conversation 绑定并在卸载时清理。主 Composer 复用人工介入队列，桌面 Thread 不重复审批面；窄屏 Thread 覆盖时保留其可达入口，不向其他群成员公开本机权限。

- 群设置本人头像来自匹配 Control 用户 ID 的当前远程登录身份；可邀请目录排除自己，不能用该目录推断本人头像为空。

- 真人私聊移出列表复用 PATCH 的 hide_direct，保留原幂等命令直到确认。仅移出本人列表，双方历史不变；重新打开或收到新消息时恢复会话。


- `team-workspace.tsx` 默认显示 Relay 群共享文件，复用公共 WorkspaceFileTree；本机 Agent 文件另列页签。上传按文件名和内容摘要生成稳定命令，不随失败重试变化；读错误不冒充空目录，上传未确认不被例行刷新清除，离开页面中止传输。当前支持有界上传/下载，不提供共享文件重命名、删除或编辑。
- `POST /team-node/room` 为当前已加入的本人 Agent 准备确定性执行 Room；不需要 Node 授权或历史任务，不启动 runtime。工作图和子智能体沿原生会话空态展示，模型权限设置在第一轮前可用。成员与本机目录交集由服务端重新核验。

- `team-execution-surface.tsx` 按已验证成员绑定复用 Room 工作图、子智能体、Agent 工作区与简介；只允许当前群 active Agent 与本机目录的交集，文件导航回到同一 Agent。本机工作区不冒充 Relay 共享目录。简介保存沿用 Agent Options 命令，工作图消费原生 execution_invalidated。

- `team-execution-thread.tsx` 在 Thread 打开前通过 TeamExecutionObserver 按绑定的 Conversation 去重订阅原生 useAgentConversation；连接和执行状态变化触发任务关联对账，流式 delta 不触发 HTTP，任务读取不设定时轮询。本机终态复用 Room Agent round 投影，避免任务持久化稍晚导致 UI 持续思考。Thread 按精确 round/Agent 读取过程与权限；窄屏复用 Room 模态外壳。远程成员不订阅本机执行流。
- Thread 活动态复用 Room Agent round 投影，不以 session/history fetching 充当执行中。精确 delivery 的 final 或本机任务终态关闭活动提示，并补读当前轮持久历史；普通中间 assistant 回复不代表执行完成。

- Relay 资源同时要求远程登录和非空 organization_id；目录刷新作用域包含组织和组织角色。组织切换废弃旧在线目录与在途读取，不切换本地 owner 数据目录。

- 入群就是本人 Agent 的群内执行授权；页面通过 `/team-node/room` 校验成员、准备本机会话并自动登记执行，不再提供独立设备授权弹窗或两步开关。凭据仅由后端持有，准备失败保留明确重试反馈。
- 本机任务链接在线群的精确 Thread，处理权限与问答；运行状态未知仍阻止重跑，不提供无证据解锁。暂停/移除/组织撤权继续由 Relay 校验，不通过自动登记绕过。
- `team-execution-thread.test.tsx` 验证精确本机轮次隔离、停止命令与历史读取重试。聊天按消息/Delivery ID 分批查询本机历史，深链额外查询精确 job；不依赖授权面板最近 100 条窗口。

- `use-team-rooms.ts` 读取已加入的在线 Room；`use-team-invitations.ts` 独立读取和处理待加入邀请，不创建默认 General。
- `use-team-room.ts` 负责快照、差量游标、WSS 水位/换代提示和真人消息提交；显式选择的 active Agent 以结构化 mention 和当前 `membership_version` 提交，WSS 不承载消息正文，提交成功后仍从旧游标走 difference 再前进。
- `use-team-refresh.ts` 统一单飞元数据读取，复用共享 directory WS，无固定轮询；读取期间的失效合并为一次后续对账。本机任务传 false，不订阅目录，沿原生 Room 事件刷新。focus、online、重连初始提示和手动重试仍可对账。Room 成员更新不重载消息历史，管理弹窗新快照即时回传聊天页，旧版本不能覆盖新版本。
- 未确认消息冻结正文、目标、成员版本与命令 ID，期间锁定草稿；再次发送只重放原意图。服务端明确 `not_applied` 才允许新命令和新版本；成员变化不能静默丢弃用户选中的 Agent。
- `team-message-outbox.ts` 发送前写浏览器持久存储，按组织/Control 用户/会话隔离，每条命令独立 key 防止窗口覆盖；保存失败不发送。重新打开只恢复未确认意图，本人 snapshot/difference 的精确 `client_message_id` 才可清除；不自动发送、不自动丢弃损坏记录。
- `team-command-outcome.ts` 区分本次未执行与先前未知提交：网关/身份拒绝不能释放先前未知命令，只有精确回执查询后的领域拒绝才能解除重试锁。
- Room 目录也使用统一可见刷新；接受或接管响应丢失时同步刷新已加入目录。403/404 撤权读取清除旧 Room、消息和 WSS。组织管理员的 `recovery_rooms` 仅为待接管元数据，确认接管后才成为成员。
- 群管理弹窗复用 Room 的 `RoomDialogColumns`、`RoomIdentityFields`、`RoomMemberDirectory`，左侧管理名称、头像和主持，右侧以人员/Agent 页签、搜索和紧凑行管理成员，切换保持目录高度。候选项直接在行尾邀请或添加；成员角色不重复显示，单一操作直接显示图标。退出、解散与移交保留确认，幂等版本继续由资源层持有，不能为重置 UI 草稿而卸载未确认命令。目录失败显示恢复入口并阻止未知 Agent 目录下添加。
- `team-stream-event.ts` 只校验当前 stream/epoch 的水位提示与显式换代事件。
- Room 可见刷新同时使用详情中的持久消息水位驱动 difference，恢复遗漏推送或失败补拉；epoch 改变才重建快照，不重发消息。
- Relay 是共享消息权威源；浏览器只访问同源 Nexus gateway，不接触 Control 或 Relay token。
- Agent 完整 assistant/final 回复独立展示，按 Control Agent 目录解析名称和头像；只有 `author_type=user` 才允许本人消息样式和发件箱对账，不能按相同真人所有者吞并 Agent 身份。远端回复不伪造流式执行态。
- 本人消息归属、成员目录排除自己、治理操作与发件箱统一使用 `control_user_id`；本地 owner key 不能用于远程成员判断。页面实例同时隔离组织与远程账号。

- retryLoad 是显式加载恢复命令，复用 reload 的快照读取与提交栅栏；单一在途 controller 防重，owner effect 清理或卸载时 abort，迟到 finally 不更改新请求状态。它不调用 postTeamMessage；发送失败继续由用户自己的发送动作处理。

- 在线 Room 不维护独立建群弹窗；`conversation/room/members/CreateRoomDialog` 统一本地与在线创建，Team 目录只提供可邀请真人和 Relay 提交资源。
- `use-team-room-members.ts` 分别以 Relay `membership_version` 和 `configuration_version` 提交成员与主持 Agent 命令；成员弹窗把 Organization 真人与当前用户本地 Agent 分区管理。只有用户明确选择的 Agent 才向 Control 发布公开身份，Gateway 校验归属后才能进入 Relay。
- 在线 Agent 暂停保留成员身份，只阻断后续投递资格；仅 Agent 所有者可暂停或恢复，暂停主持 Agent 时 Relay 同时清空主持职责。
- Agent 发布和入群由成员资源的同一个同步单飞锁持有，发布错误进入可见失败状态；删除弹窗的独立异步发布路径。读取失败保留已有快照并可手动刷新。
- `use-team-invitations.ts` 持有当前真人的 pending 邀请与幂等接受/拒绝；`team-invitation-list.tsx` 只在聊天目录展示待处理项。

- 邀请列表在无邀请、无群主接管待办且读取成功时不渲染，首次加载也不显示空标题；只在读取失败时显示重试。真人 DM 邀请卡片与目录待办复用接受、拒绝入口，群主接管仍由目录待办提供。

- Room WS 与目录 WS 的详情提示共用单飞刷新；连续提示合并补读，不并发拉群详情。详情错误受请求代次约束；切群释放同步占用，旧差量的成功、失败与 finally 均不得覆盖新群状态。

- `human-contacts-directory.tsx` 展示当前组织真人，排除自己并通过 `direct_user_id` 打开唯一双人 Relay Room；不创建本地 Agent 会话。私聊复用发送、outbox、snapshot/difference 和邀请处理，隐藏群治理与本机 Agent 执行入口。
