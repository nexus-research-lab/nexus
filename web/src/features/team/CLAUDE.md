# Team

- Relay 资源同时要求远程登录和非空 organization_id；目录刷新作用域包含组织和组织角色。组织切换废弃旧在线目录与在途读取，不切换本地 owner 数据目录。

- `team-node-dialog.tsx` 在在线群 Header 提供本机授权，复用共享弹窗与 Checkbox；待确认时锁定服务端原意图，读取失败禁用新授权。它只使用 `/team-node`，不获取机器凭据，也不把登记成功表示为执行器在线。
- 已授权宿主必须另行显式开启执行；本机任务列表链接原生执行 Room 处理权限与问答。旧授权不自动开启，运行状态未知明确阻止重跑，不提供无证据解锁。

- `use-team-rooms.ts` 读取已加入的在线 Room；`use-team-invitations.ts` 独立读取和处理待加入邀请，不创建默认 General。
- `use-team-room.ts` 负责快照、差量游标、WSS 水位/换代提示和真人消息提交；显式选择的 active Agent 以结构化 mention 和当前 `membership_version` 提交，WSS 不承载消息正文，提交成功后仍从旧游标走 difference 再前进。
- `use-team-refresh.ts` 统一可见页面的单飞元数据刷新（15 秒、focus、online 与手动刷新）；Room 成员更新不重载消息历史，管理弹窗新快照即时回传聊天页，旧版本不能覆盖新版本。M1 暂无成员变更推送，扩容时替换为版本通知。
- 未确认消息冻结正文、目标、成员版本与命令 ID，期间锁定草稿；再次发送只重放原意图。服务端明确 `not_applied` 才允许新命令和新版本；成员变化不能静默丢弃用户选中的 Agent。
- `team-message-outbox.ts` 发送前写浏览器持久存储，按组织/Control 用户/会话隔离，每条命令独立 key 防止窗口覆盖；保存失败不发送。重新打开只恢复未确认意图，本人 snapshot/difference 的精确 `client_message_id` 才可清除；不自动发送、不自动丢弃损坏记录。
- `team-command-outcome.ts` 区分本次未执行与先前未知提交：网关/身份拒绝不能释放先前未知命令，只有精确回执查询后的领域拒绝才能解除重试锁。
- Room 目录也使用统一可见刷新；接受或接管响应丢失时同步刷新已加入目录。403/404 撤权读取清除旧 Room、消息和 WSS。组织管理员的 `recovery_rooms` 仅为待接管元数据，确认接管后才成为成员。
- 群管理弹窗复用 Room 头像选择器和共享表单，支持名称/头像修改、清除主持、非群主退出与群主解散；危险操作先确认，幂等版本继续由资源层持有。
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
