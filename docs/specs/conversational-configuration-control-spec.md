# Nexus 对话配置控制面

## 目标与原则

配置真相源是异构的（数据库、用户配置目录、部署环境、原生桌面宿主，见下表）。对话配置让智能体操作真实配置能力：不允许模型随意读写文件或执行 SQL，也不维护会与数据库漂移的影子 JSON。控制面把这些真相源投影为可发现、可脱敏读取、可计划、可审计的虚拟配置树；每个写操作调用对应领域服务，Web UI、HTTP API 与对话入口共用同一套校验、级联、加密和 runtime reconcile 规则。

五项原则：

1. 身份和作用域由可信 runtime 上下文绑定，并在每次调用时从数据库重验。
2. `inspect`、`plan` 和 `apply` 使用同一套字段级与操作级授权，不存在“能看到就能改”的推断。
3. 计划绑定身份、作用域、输入和资源版本；旧计划不能覆盖新状态，也不能换目标重放。
4. 写入后重新读取真相源；成功、失败和需要 reconcile 的部分变更都留下脱敏审计。
5. 生效时机是配置契约的一部分；系统明确区分立即、下一轮、下一会话和重启生效。

## 配置域与真相源

| Domain | 真相源 | 对话入口 | Runtime 生效 |
|---|---|---|---|
| `preferences` | 用户 preferences JSON + 独立 WebSearch 凭据 | `nexuscfg` | WebSearch 立即同步；其他默认值用于后续执行 |
| `providers` | 数据库 | `nexuscfg` | 目录与检查立即；模型运行配置下一轮 |
| `agents` | 数据库 + 派生 workspace settings | `nexuscfg` | 资料/UI 立即；权限即时同步；其他 runtime 设置下一轮 |
| `emotion` | 当前 Agent workspace 的版本化 `.agents/emotion.json` | `nexuscfg` | 基础/上下文情绪下一轮投影；fatigue 只读 |
| `channels` | 数据库 + 加密凭据 | `nexuscfg`；扫码/验证码走 `nexus` MCP Channel 授权工具 | 版本 CAS 后热重载，失败条件回滚 |
| `connectors` | 数据库 + 加密凭据 | 直接凭据走 `nexuscfg`；OAuth/Device 走 `nexus` MCP Connector 授权工具 | 下一会话或重新授权 |
| `skills` | 数据库 + 用户 Skill 库 + owner catalog version + 目标 Agent `runtime_version` | `nexuscfg` | 来源、目录和导入结果立即；Agent 在下一轮加载 Skill 内容与安装选择 |
| `host` | 部署环境 + 原生桌面宿主 | `nexuscfg` 脱敏检查；变更走对应人类控制面 | 外部变更后重启 |
| `sessions` | owner-confined Agent workspace session meta + owner lifecycle ledger | `nexuscfg` | 标题/目录立即；删除先持久封锁再关闭精确热态，启动和周期恢复未完成清理 |
| `rooms` | 数据库 + Room runtime | `nexuscfg` | 资料、成员参与闸门和权限即时；提示与路由见 Room 热重载矩阵 |
| `automation` | 数据库 + scheduler runtime | Agent task 走内置 Skill + round-scoped `nexus.command`；script task 仅人类控制面 | structured inspect/plan/apply，后台 run 只读 |
| `workspaces` | workspace 文件系统 | 主智能体通过 `nexus-manager` Skill 调用 owner-scoped `nexusctl`；当前 Agent 使用原生文件工具 | 当前 workspace 文件写入立即 |
| `goals` | 数据库 + Goal runtime | 内置 `goal-manager` Skill + round-scoped `nexus.command` | 专用 Goal 生命周期 |
| `executions` | 数据库 + Execution runtime | 内置 `execution-orchestrator` Skill + round-scoped `nexus.command` | 专用 Plan / WorkGraph 生命周期 |

部署环境和桌面状态根属于宿主控制面：

- 智能体可以读取脱敏状态、运行确定性检查并解释正确操作方法。
- 智能体不能把一次文件或数据库写入伪装成已经改变当前进程。
- 服务器 workspace 根只能由部署环境配置。
- 桌面端只迁移完整状态根，并在 sidecar 退出后离线切换和重启。

### 对话入口

- `nexus_manager` 和 `nexus_config` MCP 都不挂载。
- 主智能体通过内置 `nexus-manager` Skill 调用宿主注入、owner-scoped 的 `nexusctl`；普通 Agent 不能调用 `nexusctl`。
- 所有交互 Agent 通过内置 `nexus-configuration` Skill 调用按 runtime round 签发的 `nexuscfg`。可执行操作由当前可信 DM/Room 身份决定；普通 Agent 只获得自身或当前 Room 范围，不能扩大为其他 Agent、Room 或 owner 全局权限。
- Goal 与 Automation 不是普通配置 patch，走内置 Skill 和 round-scoped `nexus.command`。
- CLI 都调用既有领域服务，模型不得直接读写数据库。

### Goal

- 创建、读取、明确改写目标和模型终态由 round-scoped command adapter 完成。
- 模型先按需加载 `goal-manager`，再通过 `contract → inspect → invoke` 读取精确 operation contract 并提交命令。
- 身份和 Goal revision 全部由 round actor 绑定。
- 暂停、恢复、预算和清除会触发 usage 结算、continuation 与当前 round 中断。它们不属于对话工具，只保留给当前认证的人类界面，也不得伪装成 `nexuscfg` 普通字段更新。

### 客户端本地状态

浏览器主题、界面语言以及 onboarding/tour 的完成、忽略和重置不是 owner/Agent/Room 配置：

- 只存在于当前浏览器 `localStorage` 或桌面本地持久状态；
- 由当前人类客户端设置；
- 不向 Room、其他客户端或后台 Agent 传播。

## 可信身份与权限边界

### 身份来源

- `nexuscfg` 不接受模型声称的 owner、Agent、Room、session 或 scope。
- 宿主向每个可信交互 runtime round 注入 `NEXUSCFG_COMMAND_PATH`、loopback broker 地址和随机 capability。
- broker 只在对应 round 仍运行且身份唯一时，把 capability 还原为当前 Agent/DM/Room Actor，再交给 configuration 服务重验。
- Hook 拒绝通过环境变量、capability 或命令行覆盖作用域；显式覆盖会失败。
- `NEXUSCTL_USER_ID` 和 owner scope 只注入主智能体。

### 内置 Skill 与渐进披露

- 只保留系统内置 `nexus-configuration` Skill，并为所有 Agent 启用。
- Skill 说明 inspect/plan/apply 流程、角色矩阵、秘密边界和按需 reference，不承担授权。
- broker 与 configuration 服务每次执行都重验 round、Actor 和资源权限；apply 在同一服务流程重新 plan 并执行 revision CAS。
- Room 成员同样使用该 Skill，但只获得当前 Room 和自身上下文允许的操作。

### 字段和操作边界

| 角色 | 可写 | 只读或禁止 |
|---|---|---|
| 主智能体 | owner 范围内的 Provider、Agent、偏好、Channel、Connector、Skill、Session 和 Room | `host` 始终只读；不能修改用户、订阅、部署环境、项目 ACL 或其他 human-only 控制面 |
| 普通 Agent | 自己的 profile、runtime、Skill、情绪与当前私聊标题 | 可用 Provider 目录只读 |
| Room 群主 | 当前 Room；当前 Agent 自己的上下文情绪 | — |
| Room 普通成员 | 当前 Agent 自己的上下文情绪 | 当前 Room 只读 |

### Sessions 域

- 只包含 owner workspace 中的普通 Agent session；Room conversation 完全由 `rooms` 域管理。
- 主智能体可重命名或删除任意自有 Agent session。
- 删除当前正在执行配置命令的 session 被拒绝。
- 删除其他 session 的顺序：在 owner state 写入 deleting tombstone 并安装精确 runtime admission fence → 关闭 runtime → 以 `configuration_version` CAS 提交 meta 删除 → 清理 transcript。
- 配置 inspect 是纯读投影，不为刷新 active 状态推进版本；返回值不包含 SDK `session_id`、resume 标识或 runtime options。

目录身份与清理：

- 历史 session 目录名编码不是单射。所有读写先核对 `meta.session_key` 与请求值，并按真实物理目录加同一把锁。
- 碰撞时 fail closed，不能借别名读取、覆盖或删除另一个 session。
- 提交后的持久 `deleted` tombstone 永久阻止同一物理身份的晚到 writer 或新 runtime 复活；Agent workspace 文件不能删除或伪造它。
- transcript 清理引用只保存在该私有 ledger，清理成功后才移除；清理失败返回 `reconcile_required`。
- 宿主启动时 fail-closed 扫描，并周期 reconcile 残留的 deleting、目录提交和 transcript 清理；不丢失重试凭据，也不把已提交的删除伪装成失败回滚。

### Rooms 域

主智能体可对 owner 范围内的 Room 执行：

- `update_profile`：名称、描述、头像；
- `set_collaboration_policy`：Room Skill、群主默认接管、私域消息开关；
- `add_member`、`remove_member`；
- `set_member_participation`：暂停或恢复指定成员的 Room 调度；
- `transfer_host`；
- `create_conversation`、`update_conversation`、`delete_conversation`。

服务端重新校验 Room 归属、成员关系和资源版本，不使用聊天文本中的身份声称。

### Automation 投递边界

Automation 使用自己的 round-scoped CLI command service，但投递权限不能绕开配置边界：

- 普通 Agent 只能把结果投递到自己的 session/inbox。
- Room 中只能投递到 runtime 绑定的当前 conversation；执行或重试前重新读取最新 task、Room 归属和当前成员关系。
- 外部 Channel 投递必须精确匹配这次可信对话已授予的目标。
- 只有主智能体自己的认证 WebSocket 私有 DM 可以签发 owner 级私有投递；旧 task 或历史成功记录不能替代当前授权事实。
- 对话入口只允许创建或管理 `execution_kind=agent` 的任务。`execution_kind=script` 任务始终属于 human-only 控制面，即使 `owner_main` 也不能在对话中创建、修改、删除、运行或修复。

## 有效配置的继承与覆盖

运行时有效配置按以下层次解释，持久写入只能修改调用者有权拥有的那一层：

```text
owner defaults
  -> Agent base
    -> current Room policy
      -> current member relationship
        -> round transient context
```

- owner defaults 为新 Agent 和未显式设置的 Agent 字段提供默认值。
- Agent base 保存该 Agent 的模型、运行上限、工具、Skill 和 MCP 选择。
- Room policy 只覆盖 Room 拥有的协作、安全和路由规则，不反向改写 Agent 持久记录。
- member relationship 决定当前 Agent 是否仍是成员、是否暂停参与，以及能否进入 Room、读取上下文和产生输出。
- round transient context 只对当前执行生效，不能被持久化成更高层权限。

合并必须满足以下单调安全规则：

- 身份、owner、主智能体标记和 host 身份不参与模型可控的继承或覆盖。
- deny/revoke 取并集；下层不能删除上层拒绝。
- allow 范围取交集；Room 或 round 只能收紧，不能扩权。
- 数值上限取更严格值；下层不能超过 owner 或 Agent 上限。
- 显式值只在该字段所属层内覆盖默认值，Room 不能借同名字段改 Agent 全局配置。
- Secret 不隐式继承到其他 Agent、Room 或 round；使用凭据必须经过显式 owner 配置和对应能力授权。

## 稳定写入协议

流程：`inspect → plan → revision → confirm → 同进程重新 plan → CAS apply → verify + audit/reconcile`。

1. `inspect` 返回调用者可见的域、操作、当前 scope、authority、脱敏状态、确定性 checks、domain revision 和资源 `state_version`。
2. `plan` 重新鉴权并验证 exact operation、target 与 input。未知字段直接拒绝，不静默忽略。
3. `plan_digest` 由配置服务在进程内绑定 owner、Agent、scope、domain、operation、target、规范化 input、revision 和 state version。CLI 不接受外部 digest，避免跨进程重放。
4. 删除、权限变化、成员变化和群主转让等高风险操作，主智能体必须先向用户展示 plan 风险并取得明确同意，才能在 apply 中使用 `--confirm`。
5. `apply` 可携带 `request_id` 和先前的 `expected_revision`。CLI 在同一进程内重新 plan 并验证 owner/main，然后按资源 scope 串行执行。旧 revision 或不同输入在写入前失败，调用方回到 inspect。
6. 写入前记录 `applying` 审计，再调用领域服务并执行数据库 CAS：
   - Agent 使用 `runtime_version`；Provider、Room、Channel、Connector 和 Session 使用各自单调版本；Preferences 使用持久化 `version`。
   - 任何有版本的 update/delete 缺少 `state_version` 都直接失败。
   - Web/API 与 CLI 写入共用 owner 锁和领域版本，任一并发功能写都会使旧 plan 失效。
7. 写入后重新读取真相源和 checks，审计记为 success、failed 或 `reconcile_required`。领域服务产生了部分外部效果但最终核对失败时，记录 `reconcile_required`，不谎报成“什么都没发生”。

### Revision 与 plan digest

- 配置 `revision` 使用 `hmac-sha256:v2:` 格式。HMAC 输入绑定 domain、scope、target、资源 `state_version` 与未脱敏值。
- 秘密轮换会改变 revision，但输出不能作为低熵秘密的无密钥离线猜测依据。
- 密钥只保存在宿主数据库的 `configuration_revision_key`，不写入 owner workspace、运行时环境、快照或审计。
- migration 142 创建明确的未初始化单例；首次快照以 CAS 安装 32 字节随机密钥，并发宿主读取数据库中的获胜值。这次私有元数据初始化不改业务配置、权限或 receipt。
- 之后只读取密钥；缺表、缺行、损坏或未知版本均拒绝，不回退临时密钥。完整数据库恢复必须保留该表。
- 同一快照可跨服务进程和数据库重开比较。
- `plan_digest` 由另一把进程临时密钥签发；重启后的旧计划仍须重新 plan/确认。
- 历史进程内 revision 的密钥不能恢复。旧 receipt 与 v2 快照返回 `revision_relation=incomparable`，不伪报配置已变化；禁止重写或重算历史 revision，禁止自动重放来猜测结果。人工仍须根据当前快照明确确认结果。
- 稳定 revision 不证明多文件事务、外部副作用结果或跨进程人工 reconcile 与所有领域写入的原子性。

### Provider 聚合

- 主记录、模型卡、默认模型和最近测试状态是同一个配置聚合，plan 公开单调 `configuration_version`，不公开凭据内容。
- 更新、模型同步、模型 patch、默认切换、测试结果和删除都先以 plan 中的 `configuration_version` CAS，再在同一数据库事务内完成；每次目标写入只推进一次版本。
- apply 后必须重新读取并证明版本从 plan 值精确推进一次（`+1`）；删除则直接证明目标不存在。
- 更新输入采用 merge-patch 语义：未提供字段保持原值，显式数组替换数组，可清除字段按契约清除。对话 merge patch 在未脱敏的最新持久化记录上合并，未声明字段不会被 plan 阶段的旧快照覆盖。
- 切换跨 Provider 默认模型时，失去默认项的 Provider 也推进自己的版本，使其旧计划立即失效。
- 普通 `verify=true` 不发网络请求。连通测试可能产生费用或外部流量，必须显式 plan/apply `test_provider` 或 `test_model`。
- 外部连通请求发生在短事务之前；请求期间版本已变化时，远端请求可能已经发生，但 CAS 拒绝任何过期结果落库。

强制删除：

- 统计所有状态（包括已归档）仍引用它的 Agent，但保留这些显式 Provider/model 绑定，不改写 Agent 记录或推进 `runtime_version`。
- 删除前必须已经存在不指向目标 Provider 的有效默认模型；Provider 删除与受影响计数在同一事务中提交。
- 从下一轮开始，运行时发现绑定 Provider 不可用时动态采用当前 owner 默认模型；同名 Provider 后续恢复时，原显式绑定可自动恢复。

## 热重载与撤权

### 工具面与执行权限

- 每个活跃 Nexus Session 先按不随轮次变化的拓扑和显式选择确定 MCP 工具面。
- 用户输入、内部唤醒、私域回传、Room host/member 角色、WorkBinding/ReviewBinding、Goal authority 和通讯开关只改变当轮执行权限，不卸载工具 schema。
- 无权轮次不签发可信 `ContextID`、human principal 或执行绑定；真实工具调用仍在 service 真相源上 fail closed。
- 后台 Automation run 是独立的受限执行 profile，不借用交互 Session 的 mutation authority。
- `ToolSearch` 默认关闭；开启时也只是 schema 传递优化，不是 MCP 挂载或鉴权机制。

### 轮次提示

- 普通 DM/Room 的 lane 权限映射只在稳定、可缓存的 Execution system prompt 中定义一次。
- 每轮动态输入只携带紧凑 `<nexus_round>` 的 `lane`、`role`，以及必要时的 `execution="background"`、`plan_only="true"`。普通轮不重复动作白名单、黑名单、Agent identity、Session key 或后台 Execution ID。
- 只有受管 Work/Review、显式 observation 或已激活 coordination 才投影完整 `<nexus_execution_context>`、当前 `allowed_actions` 与 Runtime Graph 事实。
- 两种提示都只是模型行为约束，不替代 service 权限真相源。

### 工具面变更与 SDK Session fork

正常工具面只在用户显式修改 Agent 的 MCP/Connector 默认或当前 Session 的 Connector 选择后，从下一轮生效。

宿主不能把 bridge/nxs/Claude Code 的配置下发成功视为模型已采用新 schema。在交互 DM 或 Room Agent Session 中，已有可恢复 SDK Session 的模型可见工具面指纹变化时：

1. 先关闭同一 Nexus Session 的 warm client。关闭等待必须覆盖 runtime transport 的优雅退出和强制终止两个阶段，不能在强制终止刚发出时用同长度 deadline 把换代误判为失败。
2. 再从旧 transcript 幂等 fork 一个新物理 SDK Session，使当前 MCP 工具面从该分支首轮起成为启动事实。
3. target SDK identity 由宿主稳定派生，连接失败可重试同一分支。nxs 可在 Connect 阶段确认该 identity；Claude Code 可能直到首条 query 的 init 事件才公布。宿主必须在相应确认点验证它不是 source identity。
4. 新 identity 与工具面指纹只有在 transcript 可恢复后才能共同提交；Room 成员 Session 的两者必须在同一 SQL 事务中持久化。
5. 失败时保留旧 identity 和旧基线、关闭未提交的 fork client，且不得回退到旧 warm client 执行本轮输入。

- Nexus Session key、标题和可见历史保持不变。fork transcript 自带旧上下文；旧 identity 只保留为清理 lineage，不作为并列可见分段。
- 该规则不依赖 K3、nxs 或 Claude Code 的名称。只有明确协商并确认会话内动态工具更新的 runtime capability 才能绕过 fork。

DM Connector 选择变化时的后台预备：

- 当前 DM Session 的有效 Connector 选择变化且已有可恢复 transcript 时，设置事务提交后以短 debounce 启动 latest-wins 后台预备。
- 同一 Session 只保留最后一个配置版本；过期预备必须取消；只有配置版本仍匹配的 fork identity 与工具面指纹可以提交。
- 真实输入到达时取消尚未提交的预备，并立即进入同一 runtime 启动主链。
- 预备失败不回滚设置；发送时同步 fork 是正确性兜底。
- 后台预备不得发送隐藏模型消息；无法在 Connect 阶段公布新 identity 的 runtime 留到真实首轮完成 fork。

### Connector 状态投影

交互 DM 的动态模型上下文分别投影 owner 级 Connector 配置/授权状态与当前 Session 的有效选择状态，二者不得互相推断。该宿主快照就是状态真相，模型不得再调用 `nexusctl` 或授权工具重复确认。

| 状态 | 模型行为 |
|---|---|
| 已配置、未选择 | 只提示用户点击对话框左侧的「+」，在弹出菜单中为当前 Session 选择该 Connector；不得重新发起授权或模糊指向不存在的“会话设置” |
| 已配置、已选择、schema 存在 | 以当前 schema 为准，不得沿用旧轮次的“没有工具”结论 |
| 已配置、已选择、schema 缺失 | 报告 runtime 挂载异常，不得谎报为未配置或未授权 |

状态投影只包含 Connector ID 和脱敏状态。实际 MCP 装配完成后，宿主可在当前轮补充已挂载 server alias 及宿主已知 Connector 的精确工具名，但不得携带凭据或工具 schema 正文。

### 生效矩阵

“热重载”不是一个布尔值；不同配置按安全要求和 runtime 生命周期分级：

| 变更 | 持久化后生效 | 活跃执行处理 |
|---|---|---|
| 初始化 owner / 启用服务端认证 | admission gate 完成撤销后原子提交；后续启动按 `NEXUS_RUNTIME_ISOLATION_MODE` 选择隔离模式 | 阻断新启动，取消并排空在途 DM/Room/AutoDream admission，关闭 system owner 的既有 session/round；任一步失败都不提交认证 |
| Agent 名称、头像、描述、标签 | UI/目录立即；prompt 下一轮 | 当前输出保持本轮身份快照 |
| Agent `permission_mode` | 当前 DM 与 Room runtime 立即同步 | 后续工具授权立刻使用新模式 |
| Agent Provider/model、运行上限、tools、Skills、MCP | 下一轮重建 client options；显式 MCP 修改才更新 Session 工具面 | 不在半轮中替换模型或工具表 |
| Provider 主配置、模型卡、默认选择、测试状态 | Provider 目录和检查立即；引用它的 Agent 下一轮重建 client options | 当前 round 保持已捕获的 Provider 配置；写后核对见 [Provider 聚合](#provider-聚合) |
| 强制删除使用中的 Provider | 保留显式绑定并使不可用选择动态回退 | 当前 round 不切换；下一轮起见 [Provider 聚合](#provider-聚合) |
| WebSearch 凭据/设置 | 活跃 nxs runtime 立即同步 | 同步失败仅在 version 仍等于本次写入时回滚；若已有后续写入则保留新状态并报告 reconcile |
| Channel 配置、账号 | 候选 Channel runtime 热重载（见 [Channel 热重载](#channel-热重载)） | 删除或禁用后新入口立即拒绝 |
| Channel pairing | 数据库 CAS 后由下一条 ingress 重新查询生效 | 停用或删除后下一条外部消息即被拒绝或重新进入配对流程 |
| Channel QR/验证码授权 | 启动版本 CAS 后发布候选 runtime | QR/验证码只进入绑定的认证 UI；候选失败保留旧 runtime，旧 generation 不能迟到覆盖 |
| Connector 凭据/连接 | 下一会话或重新授权 | 不把旧会话伪装成已换凭据 |
| Connector OAuth/Device 授权 | 完成时按启动配置版本 CAS | OAuth URL 由受保护的 `flow_id` 跳转恢复；跨 owner/session、过期或并发变更拒绝 |
| Skill 来源、导入、更新和安装选择 | 私有来源增删改、搜索、目录和导入结果立即；目标 Agent 下一轮加载内容与选择 | 所有 Settings/API/CLI 功能写共用 owner catalog CAS；Bearer 仅通过 Settings 或人工 CLI secret slot 输入；发布失败原子恢复旧目录或进入明确 reconcile |
| Scheduled Agent task / Heartbeat | scheduler 读取持久新版本；wake 不改变配置版本 | 更新/删除用版本 CAS 并重读；wake 与配置版本在同一事务栅栏内受理并先写 durable outbox，同 owner/request/intent 只重放同一回执；重启只恢复未领取 wake，已开始但结果未知的 claim 不自动重投；script task 不开放对话写入 |
| Agent session 标题 | 目录/UI 立即 | 同一 session 资源锁内单调推进版本；写后重读标题 |
| 删除 Agent session | owner lifecycle ledger 先封锁，meta 删除后保持 tombstone | admission fence 阻止新启动和晚到写回；关闭失败撤销未提交栅栏；提交后的清理失败见 [Sessions 域](#sessions-域) |
| Agent 基础/上下文情绪 | 下一轮稳定投影 | 版本 CAS；只改变当前 Agent 自有状态，不动态改写半轮 prompt |
| Room 名称、标题、头像、描述 | Room UI/目录立即；稳定 prompt 下一轮 | 当前 round 使用已捕获的展示快照 |
| Room Skill | 下一轮稳定 prompt | 不在半轮中替换协作规则文本 |
| `host_auto_reply_enabled` | 下一条输入路由 | 当前已派发 slot 不改目标 |
| `private_messages_enabled=false` | 服务层立即撤销，工具 schema 保留 | 每次私域工具调用重新读库，旧 prompt 也无法绕过 |
| `private_messages_enabled=true` | 服务层立即允许，工具 schema 不变 | 当前 client 不动态改工具表 |
| 添加 Room 成员 | 成员目录与后续路由立即 | 新成员从后续输入开始获得 slot |
| 移除 Room 成员 | 权限立即撤销并中断活跃任务 | 最终输出前再验成员关系，旧 runtime 在途结果不能落库 |
| 暂停/恢复 Room 成员参与 | Room CAS 与 authority epoch 立即推进；暂停中断活跃任务，恢复重启待办调度 | 最终输出前同时复核 epoch、成员关系和 participation gate |
| 转让 Room host | authority 立即变化；下一条输入使用新 host 路由 | 旧 host 的 inspect/plan/apply 在下一次调用时失败 |
| 创建/更新 conversation | Room 版本推进；目录立即 | 后续输入/下一轮读取新标题和上下文 |
| 删除 conversation / Room | 数据库先提交删除与版本边界 | 随后关闭精确 runtime、清理 artifact/Goal；清理失败记录 reconcile，不伪装成未删除 |
| 删除 Agent | Agent tombstone/CAS 先提交 | 阻止新连接和重配，撤销该 Agent 的 DM/Room runtime，再清理 Channel 引用；部分清理进入 reconcile |
| Host deployment/native state | 对话只读检查；在人类控制面变更后重启 | 不存在可由 Agent 写入的 shadow runtime settings |

### Channel 热重载

- 串行边界是 `owner_user_id + channel_type`。配置、账号、扫码登录落库、删除和 runtime 替换不能互相越过。
- 新候选必须先成功 `Start`，再以单调 generation 发布；旧 generation 的迟到完成不能覆盖当前实例。
- 启动失败保留旧 runtime，但回滚不把版本写回旧值：失败写入 `N+1` 后以新版本 `N+2` 发布旧内容，失败前后的旧 plan 都不能重新命中。
- error 同时返回配置调用方和 runtime 状态，使 apply 进入可见的 reconcile/failed 路径。
- Pairing 不参与 runtime 替换：人工更新只 patch 明确字段，并与 ingress 的 `last_message_at` writer 共用 owner 锁；每条新 ingress 都重新查询数据库，修改从下一条外部消息生效。
- 配置、账号和 pairing 删除还会直接查询各自持久化记录，证明目标不存在。

### 撤权顺序与 reload 状态

- 安全和身份撤销永远先于 prompt 重建。旧 runtime 即使仍持有上一轮文本，也无法通过服务层继续私域发送、修改 Room，或在被移除后提交最终输出。
- 每次 apply 返回结构化 `reload_status`，指出已经同步的 runtime、下一轮/下一会话动作以及是否需要重启；调用方必须把它和写后 checks 一起告诉用户。

## Secret 与自定义 MCP 边界

### 脱敏与 secret slot

- 命令输出、snapshot、plan、checks、revision 投影、审计 request/result 共用递归脱敏规则。
- token、secret、password、API key、认证 header、数据库 URL、私钥、内部 system prompt、Provider options 与 `mcp_servers` 只暴露 `configured`/redacted 状态，不回显原值，也不能把 hash 当作可恢复值。
- 模型永远不能把秘密明文放进 `nexuscfg --input`。敏感字段只能提交 `{"$secret":"opaque_slot_id"}`；slot 进入 plan digest。
- 真实值只能由用户在 Settings 中填写，或在自己的终端手工通过 `--secrets-stdin` 提供；Agent 不得使用该参数。
- 直接明文、未知 slot 和重放都会被拒绝；终态输出、transcript、日志和审计只保留 slot/`configured` 状态。
- Channel catalog 中标记为 secret 的字段会被普通 config JSON 拒绝；公开视图也过滤历史脏数据中的同名字段。

### 真人授权与 human-only 管理面

对话可以发起授权，但不能代替人类完成授权。两条专用链都只注入主智能体自己的 WebSocket 私有 DM，并绑定 owner、主 Agent、业务 session/root round、真实 runtime lease、当前认证 principal/session、启动资源版本和过期时间：

- `nexus.connector_authorization`：
  - `action=start` 必须先经过当前 permission 卡的真实 `allow`。
  - OAuth 工具结果只返回 Nexus 本地受保护路径；浏览器请求仅携带 opaque `flow_id`。服务端从 durable flow 恢复全部身份并再次验证认证 session，再 303 到 provider。
  - Provider state、PKCE、device code、auth code 和 token 不进入模型。Device Flow 只返回 provider 明确定义为公开的人类 user code / verification URI。
- `nexus.channel_authorization`：
  - action 结果只含 flow 状态。
  - QR payload、verification URL 和验证码输入只通过与原始业务 session、同一 principal 绑定的原生 WebSocket 卡片展示/提交。
  - 验证码在 wire map 中立即移除，不进入 transcript、MCP 参数、数据库或审计。
  - 重连、跨 sender、跨 lease、过期 token 和旧进程 generation 均拒绝。

以下项目不进入 `nexuscfg` 写入面，继续遵循各自已有的原生、宿主或认证管理面，不能借对话配置绕过所有权、真人确认或秘密输入边界：

- 用户/密码/角色、订阅与公共 Provider、项目 ACL；
- 部署环境和认证开关；
- Automation script task；Goal 暂停/恢复/预算/清除；
- 当前客户端的主题/语言/onboarding/tour 状态；
- 当前 Composer 发起的 Session 级模型/权限/Connector 覆盖；
- 任意本地路径或上传式 Skill 导入；
- 其他 Agent workspace 写入和直接 SQL。

### Agent 自定义 MCP

自定义 Agent MCP 已实际接入 DM 和 Room runtime 的 client options：

- 仅 `owner_main` 可以通过 Agent `update` 管理自由格式 `mcp_servers`。
- `agent_self` 不能编辑 MCP server、header、OAuth 或凭据。
- stdio、HTTP 和 SSE 配置在进入 runtime 前严格解析；未知类型、SDK 内部 server、`nexus_*` 保留名和内置 server 冲突会被拒绝。
- 修改后的 MCP 配置从下一轮生效，当前半轮不会动态替换工具集合。

### Connector 与 owner 级自定义 MCP

存储与启用：

- 应用市场 Connector 与连接器目录中 owner 级自定义 MCP 都不写入自由格式 `mcp_servers`。自定义 MCP 复用 Connector 加密仓储，以动态 Connector 参与既有选择链路。
- `connector_connections.enabled` 是 owner 可用性的持久真相。新建配置默认开启；旧记录由版本化 migration 一次性补为开启，预发布版本已写入的关闭状态保留。
- 关闭后配置和秘密保留在管理目录，但从对话选择目录移除，runtime 必须拒绝挂载。
- 标准 Connector 是主目录快照，自定义 MCP 只是可独立失败的动态附加项；旧密文或单条动态配置异常不得把标准目录替换成全页错误。

密文恢复：

- schema migration 只迁移可证明的结构与状态，不猜测或重建已丢失的加密密钥。
- 单条历史密文无法读取时，管理目录返回保留原 Connector ID 和 owner 可用性事实的 `recovery_required` 投影；不删除、不伪造配置，也不把列表当作空。
- 待恢复记录不得进入对话选择、runtime 或 Tools 发现，也不得单独切换启用状态。
- 用户可以删除它，或明确提交一份完整配置原位替换旧密文；恢复后继续使用原 Connector ID 与先前启用状态。

认证与管理详情：

- 远程自定义 MCP 只暴露无认证、Bearer Token 和自定义请求头。Bearer Token 由宿主生成 `Authorization` header；尚未形成完整登录闭环的 OAuth 元数据不进入用户配置。
- 管理详情可以用已保存认证只读连接 HTTP/SSE 服务，投影初始化信息和 `tools/list` 的工具目录；不请求或展示 Prompts/Resources，也不得以宿主身份执行 stdio 命令。
- RichMail 是固定本机 Connector：宿主对固定 loopback 端点发起无 Token 配对；浏览器只持有 owner/版本/过期时间绑定的 opaque attempt；批准后由后端按配置版本 CAS 加密保存 Bearer Token；同一 attempt 的成功重试只凭持久 exact receipt 对账。

选择与挂载：

- Agent 的 `connector_ids` 只保存 owner 已开启集合内的默认挂载选择，默认值为空。
- Composer 可以为当前 Session 显式覆盖；未设置时继承 Agent，空数组表示全部关闭。
- 显式选择决定对应 Provider 或自定义 MCP 的 Session 工具面。飞书云文档使用独立的 `nexus_feishu_docx` MCP；RichMail 使用独立的 `richmail` HTTP MCP。
- 短暂未连接或凭据不可用时，宿主管理的固定工具面保留定义，真实调用返回“未连接”或具体认证错误；需要凭据才能构造的第三方远程 MCP 只在授权快照可用时建立连接。
- 未选择或 owner 已关闭的 Connector 不注入任何工具定义；不存在可绕过这条边界的通用 Connector 调用入口。

### Connector 加密密钥

Connector 数据库与宿主 keyring 构成不可拆分的加密身份。

密钥来源：

- 所有宿主只在启动入口解析密钥来源；Makefile 和业务 service 不直接访问 Keychain、文件，也不决定来源优先级。
- 一把 active key 是唯一 writer。Keychain、canonical `app/config/connector-credentials.key`、显式配置及旧 `config/` 文件中其余可验证且不同的密钥只作为有界 legacy reader。
- 桌面宿主显式注入已解析的 active/legacy 集合。
- 本地开发宿主读取同一 canonical `NEXUS_STATE_ROOT` 时，由统一解析器按 macOS Keychain、canonical 文件、显式环境的顺序选择。
- 服务器/容器默认只接受显式配置。
- 首次建立 Keychain 条目时，先导入 canonical fallback，再兼容旧 `config/` 路径，最后才生成新密钥。
- 状态根迁移必须连同文件回退密钥一起迁移。

密钥身份迁移：

- `connector_connections.credentials_key_id` 与密文同事务保存；key ID 只取密钥的 SHA-256 身份，不包含秘密。
- 启动时在 schema migration 后、业务 service 前逐条迁移：
  - 已有 key ID 只查精确密钥；
  - 无 key ID 的 v1 历史密文才按显式 keyring 有限尝试；
  - 匹配 active key 时只以原密文 CAS 补身份；匹配 legacy key 时以 active key 重加密，并 CAS 更新密文和身份。
- 每条记录独立、幂等，不改业务 `updated_at`；并发冲突留待下一次启动重试；单条未知密钥或损坏载荷不删除、不猜测、不阻断其他记录。
- 没有独立 key ID 列的 OAuth client、Connector/Channel 授权中间态和 Channel credential 使用自带 key identity 的 envelope 写新载荷，只为旧 v1 载荷走同一有限 keyring。
- 只有所有显式历史密钥都无法读取时，持久配置才进入上述逐条恢复流程。

## CLI 与审计

- `nexuscfg inspect` / `plan` / `apply`：见[稳定写入协议](#稳定写入协议)。`plan` 返回风险、确认要求和 runtime effect，不写入；`apply` 返回写后 snapshot、checks 与 `reload_status`。
- `nexuscfg history`：查询当前 Actor 有权查看范围内的脱敏审计和 reconcile 状态。
- `nexuscfg review --request-id <id>`：读取一条配置 receipt，并在同一 owner/scope 下重新读取当前脱敏真相源；只返回 revision 关系和 checks，不改变状态。
- `nexuscfg reconcile --request-id <id> --decision applied|not_applied --observed-revision <revision> --confirm`：只能由当前 owner 的人工配置入口提交，把 `reconcile_required` 收口为带 `human_confirmation` 证据的 `reconciled`。它不重放原始请求、不修改配置值，也不接受 Agent round capability 的提交。

审计规则：

- `request_id` 是审计唯一键。每次新 CLI 执行使用新 ID 或由命令自动生成。
- 结果不确定时先查询 history，再用 `review` 取得当前 revision，并由真人显式 `reconcile`；不用旧 request ID 发起新的副作用进程。
- CLI 作用域由宿主环境和当前 owner 的主智能体共同绑定。审计记录保留 owner、Agent、scope、request ID 和脱敏 intent digest，不依赖模型提供运行时身份。
- 审计读取沿用同一作用域：主智能体只查看宿主绑定 owner 的私有记录；Host 与公共管理记录仍要求 local single-user 或真实 owner/admin。

## 当前存储限制

- Provider `auth_token`、私有 Skill 来源 Token 与 Agent 自定义 MCP（`mcp_servers`）中的秘密沿用各自现有存储模型，尚未进入统一加密存储；Channel 和 Connector 凭据使用既有加密仓储。控制面协议不得返回这些明文。
- 包含外部副作用的秘密变更、OAuth 与 Channel 连接不承诺一键回滚；失败后使用重新授权、显式重配或 `reconcile_required` 收口。
- 这些限制不改变服务端身份绑定、资源 CAS、幂等 apply、写后核对、输出栅栏和全链路脱敏要求。

## 实现约束

- `service/configuration` 按领域聚合操作输入、校验、执行与核对，共用授权、批准、CAS 和审计。
- `service/orchestration` 命令在同包内按业务归组。
- `runtimehook.Observer` 统一 DM/Room 运行观察；可信会话身份仍由各宿主提供。
