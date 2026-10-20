# 在线账号、Team 与 Node 规范

> 当前维护者合同；协议字段以 `internal/relay` 与 `internal/protocol` 为准。

## 账号、组织与订阅

- 远程账号可无组织；平台 `role` 与 `organization_role` 独立。组织入口位于账户设置，非运营管理员专属。
- Relay 必须同时具有远程登录与组织身份；组织变更不能切换或清空 App 本地用户数据目录。

- User、密码和 Session 的权威位于独立 `nexus-control`。
- Nexus Web Shell 把 `/auth/v1` 的登录、登出、资料、改密、首次初始化和成员管理请求同源发送给 Control。
- Nexus Server 只验证短期签名 Principal，用 `local_owner_bindings` 把 Control 身份确定性映射到本地 owner key；展示资料只投影到 `owner_profiles`。
- Nexus 缓存短期签名 Principal 租约，有效期内本地验签；过期后重新向 Control 换取，Control 不可用则拒绝访问。
- 每个 Nexus 副本独立按游标顺序消费 Control 的持久身份失效序列；未绑定本地 owner 的事件不产生本地动作：
  - `session_revoked`（登出）只丢弃 exact browser Session 的租约并关闭该 Session 的 WebSocket；
  - `profile_changed` 丢弃 owner 租约并关闭其 WebSocket 以重连刷新，保留 Agent runtime；
  - `principal_changed`（角色变更或停用）丢弃 owner 租约、刷新本地角色/状态投影，并关闭该 owner 的全部 WebSocket 与 runtime；
  - `entitlement_changed` 丢弃 owner 租约并刷新额度投影（见下文）；`organization_changed` 只丢弃 owner 租约，协作撤权由 Relay 执行；两者都不关闭连接或 runtime。
- 失效序列持续不可读一分钟，或单个事件重试三次仍无法应用时，Nexus 把全部 entitlement 投影标为不可用、清空租约，并关闭全部 Control 身份连接与已绑定 owner 的 runtime（失败关闭）。
- 旧 `users`/认证表只允许迁移代码读取，不再属于运行时账号系统。

- Desktop 后端始终使用 `__system__` 本地主体。
- App 默认把账号 `/auth/v1` 与多人 `/nexus/v1/team` 同源代理到 `NEXUS_REMOTE_URL`（正式环境 `https://app.nexusos.cn`），由线上 Gateway 访问 Control 与 Relay；不能把 Control 服务凭据或 internal API 暴露给安装包。
- 登录只增加在线能力，不切换本地数据目录。

- 服务端 Web 的订阅套餐与成员 entitlement 写权威位于 `nexus-control`。
- Nexus 只保存 `owner_entitlements` 本地投影、持久 Control 事件游标和自身 token 用量，并在新 runtime 请求前按投影校验额度。
- `entitlement_changed` 只刷新投影，不中断正在执行的 Agent。
- 旧 `subscription_plans`/`user_subscriptions` 只允许迁移读取。
- 运营页中的公共 Provider 与项目 ACL 仍属于 Nexus 运行资源，不迁入 Control。
- 个人设置用量由 usage ledger 提供累计汇总与最近 365 个 UTC 自然日的 daily 聚合；图表和明细表共用该数据，不增加另一套记账来源。

- 统一通过 `nexuscfg members`：runtime broker 展示绑定当前真人 Session 的确认卡片，再调用 Control 内部成员 API。
- 只向有效管理员的主智能体 DM 开放；不恢复 `nexusctl auth/user`，不把 Control 服务凭据传给命令参数。
- `members.remove` 撤销部署访问并保留数据。

## 在线 Team Room

- 浏览器 API 固定使用 `/nexus/v1/team/...`，只接收现有 HttpOnly Session。
- Nexus 从认证上下文读取已验证且带当前 Organization 的 Control Principal，经 `/internal/humans/verify` 换取固定 `nexus-relay-user` 短令牌后访问 Relay。
- 浏览器正文、查询和 Header 都不能指定 user、deployment、organization 或 audience。
- `NEXUS_RELAY_URL` 留空时不挂载 Team 路由；已挂载时仍只接受有效 Control 远程 Session，Desktop 本地主体返回 403。

- 只有同一 Organization 的成员能创建和访问彼此的在线 Room。
- 创建和后续邀请都先由 Nexus Gateway 向 Control 校验目标真人；Relay 中 selected member 先为 invited，接受后才获得 active 访问权。
- 在线 Room 由用户显式创建，创建者是唯一真人群主；群主可改角色和原子移交真人群主；管理员按层级邀请、撤销和移除成员。
- 全部成员命令携带幂等键和 `membership_version`。
- 建群或添加 Agent 时，浏览器先把选中的本地 Agent 公开身份幂等发布到 Control，Gateway 再校验这些 Agent 均属于当前真人和 Organization；Relay 只保存 Control Agent ID 与 owner 投影。
- Agent coordinator 只负责分发任务，不能取得真人治理权。
- 群设置、退出、解散与组织管理员接管孤儿群由 Relay 鉴权；Nexus 不本地猜测治理权限。

- 群聊消息只有显式 `@Agent` 或结构化目标才启动 Agent；未指定目标只持久化和广播。DM 保留唯一 Agent 回退。

- 非零同步游标和快照续页必须原样传递 Relay 返回的 `stream_epoch`；世代失配由 WSS `stream.reset_required` 或 difference 触发全量快照。
- WSS 只转发提交水位；正文由 difference/snapshot 恢复。
- Relay 是在线 Room 与消息的唯一权威；Nexus 只做鉴权 gateway，不保存消息副本、`room_seq` 或同步游标（00158 删除旧投影表）。
- Relay 已提交的 message mutation 保持成功；Browser 必须从原 cursor 走 difference 补齐后再推进。

- 元数据轮询由 `web/src/features/team/use-team-refresh.ts` 统一管理；成员治理新快照回传聊天资源。
- 消息未知结果保留完整幂等意图，不随快照刷新更换目标或版本。
- 发送前由 `features/team/team-message-outbox.ts` 按 Organization、Control User 与 Conversation 持久保存命令；不同窗口使用独立命令键；恢复不自动重发；快照按本人精确回执对账。
- Room 明确撤权时立即清除聊天资源和连接。

- 在线 Agent 完整回复由 Relay 消息携带 `author_agent_id/delivery_id/output_kind`；`author_user_id` 只表示真人所有者。
- UI 只有 `author_type=user` 才进入本人消息与发件箱确认路径；Agent 显示独立成员身份。
- Node 授权和投递租约属于 Control/Relay；`service/team/node_executor.go` 主动领取并复用 Room runtime，不能把 pending/leased 直接显示为执行成功。

## Node 授权与执行

- `/nexus/v1/team-node` 是本机授权入口，不在 Desktop 的 `/team` 远程代理内。
- 服务端以当前远程 Cookie 验证账号/组织，再把本人已发布 Agent 与本机 owner Agent 目录取交集。
- 设备凭据用现有宿主 keyring 加密后先落盘；Cookie 只留哈希。
- 未知注册只重试原意图；撤销先冻结本机授权，精确回执更新本地状态；不能把未知注册的 404 当作撤销证明。
- 授权只代表设备已登记，不表示 worker 已上线。

- Node 执行另需显式开启，旧授权默认关闭。
- 机器凭据固定原 Control 地址；后台无浏览器 Cookie；最多八个并行本机 Agent。
- `storage/teamrelay/jobs.go` 持久 inbox/outbox；ready→running CAS 与撤销共锁授权行；未知 running 不自动重跑。
- `room/relay_execution.go` 按 owner/在线作用域/Room/本机 Agent 绑定独立执行会话，复用原生权限、问答与 exact round 中断。
- `node_runtime.go` 只转发 durable 且 `is_complete` 的 assistant 文本：单条 64 KiB、单任务 1 MiB；final 和本机 draining 原子落盘；重试只重放原 output ID。
- 尚未验收/实现：真实双节点模型、未知运行人工解锁和远程产物。
