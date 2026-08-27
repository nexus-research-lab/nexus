# 失败解释与恢复基础协议

## 1. 文档边界

本文定义 Nexus 当前可选的 `FailureCore v1` 失败事实、HTTP 兼容写出、前端宽容解析，以及后续业务接口接入时必须遵守的身份、证据、权限和性能边界。

`FailureCore` 只增强失败的解释能力，不替代领域状态机，也不改变现有接口。尚未显式改用 `WriteError` 的 Handler 继续返回原 `WriteFailure` envelope；Conversation、Configuration、Automation、Goal 和 Execution 继续使用各自已经存在的身份、回执和恢复协议。

本文不定义公共第三方 API，不建立全局操作 ID、全局幂等账本、全局异步流程表、前端离线写队列或全产品 WebSocket 事件日志。

## 2. 三类事实必须分开

用户界面要回答三件事，但三者不能从同一个错误字符串推断：

1. **发生了什么**：由稳定 `failure.code`、粗粒度 `category` 和领域阶段回答。
2. **本次请求对数据有什么影响**：只由事务、持久 ACK、revision、领域回执或第三方权威回执证明。
3. **现在能做什么**：由领域规则投影可选 `resolution`；公共层不得根据 HTTP 状态自行制造“重试”。

超时、断线、WebSocket `onerror` 或没有收到响应只证明传输结果未知，不能证明业务未受理、未写入或已回滚。结果未知时必须先读取权威状态或领域回执，不得自动重放可能有副作用的命令。

## 3. 身份角色与既有字段

公共可靠性规则定义身份角色，不统一现有字段名，也不允许同名字面值跨领域关联。

| 现有身份 | 当前语义 | 不得用于 |
| --- | --- | --- |
| HTTP `X-Request-ID` / `failure.transport_request_id` | 一次 HTTP 传输尝试的诊断关联 | 幂等、授权、路由、缓存或业务主键 |
| Conversation `client_request_id` | 一次 WebSocket 发送尝试的 ACK / timeout 关联 | 逻辑消息、round 或 durable 输入身份 |
| Conversation `client_message_id` | 同一逻辑用户输入和 optimistic 消息的稳定身份 | HTTP 诊断、canonical `round_id` |
| Configuration `request_id` | owner scope 内绑定 plan digest 的审计与幂等身份 | HTTP Request ID 或全产品唯一身份 |
| Automation create / command `request_id` | 各自领域 ledger 内的可选创建或命令身份 | permission、manual run 或 HTTP 诊断身份 |
| Automation manual run `request_id` | 同一 owner 下绑定 job、配置版本和启动意图的持久身份 | HTTP 诊断、创建、CLI command 或 permission 身份 |
| Permission `request_id` | 一次人工介入的持久生命周期与响应路由 | 命令幂等或传输诊断身份 |
| `run_id`、`round_id`、Goal/Execution receipt identity | 对应领域的权威流程身份 | 每次网络尝试或跨域全局身份 |

当前 HTTP middleware 只接受长度不超过 128、由 ASCII 字母、数字、点、下划线、冒号或连字符组成的 `X-Request-ID`；缺失或不合法时静默生成一次，不因此拒绝业务请求。它把同一值写入响应头、request logger 和私有 request context。Failure 写出只能读取该 context，不得从请求正文或领域对象猜测诊断 ID；没有 middleware 的直接调用不另造第二个 ID。

即使两个 POST 使用同一个 `X-Request-ID`，也必须继续执行两次，除非该业务领域本身另有经过验证的幂等合同。

## 4. FailureCore v1

结构化失败保持现有 HTTP envelope，只在 `data` 中增加可选字段：

```json
{
  "code": "409",
  "message": "failed",
  "success": false,
  "data": {
    "detail": "请求无效",
    "request_id": "http-attempt-1",
    "failure": {
      "version": 1,
      "code": "workgraph.revision_conflict",
      "category": "conflict",
      "effect": "not_applied",
      "transport_request_id": "http-attempt-1",
      "resolution": {
        "actor": "user",
        "action": "workgraph.refresh_editor"
      }
    }
  }
}
```

`data.request_id` 是现有 Web 解析器的兼容投影，与 `failure.transport_request_id` 表示同一个 HTTP 诊断值；它不是第二种身份。新业务代码只读取含义明确的 `transport_request_id`。

### 4.1 字段合同

| 字段 | 合同 |
| --- | --- |
| `version` | 当前固定为 `1`；客户端遇到未来版本必须安全退化，不得崩溃 |
| `code` | 带领域前缀的稳定开放字符串，例如 `workgraph.revision_conflict` |
| `category` | 粗粒度开放字符串；当前已知 validation、authentication、authorization、not_found、conflict、rate_limited、unavailable、timeout、canceled、internal |
| `effect` | 对本次请求数据影响的证据结论；未知值安全退化 |
| `transport_request_id` | 当前 HTTP 尝试的诊断 ID，可省略 |
| `retry_after_ms` | 只有 429 或明确 503 且服务端知道等待时间时提供，同时设置 `Retry-After` |
| `resolution` | 领域明确提供的 actor + action；action 是稳定语义名，不是 URL、命令或自由文本 |

`detail` 是普通用户可理解的安全兜底文案，客户端不得解析其中词语决定权限、重试、清缓存、导航或数据影响。内部错误、SQL、路径、堆栈、Provider secret 和原始请求正文不得进入响应。

### 4.2 effect 证据

| effect | 含义 |
| --- | --- |
| `not_applicable` | 当前请求是读取，没有写入语义 |
| `not_applied` | 有权威证据证明本次修改没有提交 |
| `accepted` | 命令已被耐久受理，但业务尚未完成 |
| `committed` | 目标领域的权威状态已经提交 |
| `unknown` | 当前没有足够证据判断 |

多阶段操作不得用单个 `partial` 隐藏阶段差异。执行、结果保存、Nexus 投影和外部投递必须由对应领域状态分别表达；执行成功但投递失败时只能重试投递，不能重新执行任务。

普通 HTTP 错误一旦发生在本地提交之后，就不能再把整个操作投影成普通 `not_applied`。调用方应获得提交回执、可查询的持久状态，或在无法确认时进入 `unknown` / 领域 `needs_attention`。

## 5. 写出与解析边界

`internal/handler/shared.WriteFailure` 保持旧行为，不自动增加 `failure` 或 request ID。新 Handler 必须显式选择 `WriteError` 并提供已经确认的 code、category、effect 和可选 resolution。

公共 writer 可以做的事只有：

- 复用现有 gateway 安全文案；明确的 `context.Canceled` 投影为 499，明确的 `context.DeadlineExceeded` 投影为 504，两者不得按错误文案混淆；
- 从 request context 读取同一个 HTTP 诊断 ID；
- 写入 FailureCore v1；
- 对空 code/category/effect 做安全兜底；
- 只在明确 429/503 时投影 `Retry-After`。

公共 writer 不得：

- 从 `err.Error()` 或 `detail` 文本猜业务 code、数据影响或恢复动作；
- 调用业务服务、数据库或外部 Provider；
- 把 HTTP Request ID 传入领域命令；
- 根据未知 `resolution.action` 自动执行操作。

领域错误到 FailureSpec 的映射留在对应 Handler 包。Service 不依赖 HTTP status 或 `internal/handler/shared`。

Web 解析器同时接受旧 `{data:{detail}}` 和新 `{data:{detail,failure}}`。`ApiRequestError` 保留既有 `name`、`message` 和 `status`，只增加可选 FailureCore 与 `transportRequestId`；未知 version、code、category、effect、actor 或 action 不得使响应解析失败。

没有拿到完整 HTTP 响应时，Web 只能生成本地 `ApiTransportError`，不能伪造服务端 FailureCore：

- GET、HEAD、OPTIONS 的传输失败是 `not_applicable`；
- 其他方法的传输失败是 `unknown`，包括连接阶段和响应体读取阶段的超时或中断；
- 调用方主动 Abort 保持原取消异常，不进入用户失败面；
- 通用请求层不重试，也不解释或执行服务端 `resolution.action`。

全局鉴权和桌面会话 token 中间件只有在未调用业务 Handler 时，才可把读取标为 `not_applicable`、把修改标为 `not_applied`。panic 或已经进入业务阶段的错误不得套用此前置结论。

## 6. 资源、修改和访问状态

接入 FailureCore 的页面必须把下列状态分开：

- 查询：首次加载、后台刷新、失败；
- 数据：无快照、fresh、stale；
- 修改：提交中、accepted、committed、not_applied、outcome unknown；
- 访问：allowed、authentication required、revoked；
- 长流程：领域当前阶段和允许动作。

同一 scope 内的暂时读取失败可以保留上次成功快照。owner、Agent、Room、Session、资源 scope 或认证代次变化时，旧请求结果必须被丢弃；401、明确权限撤销或资源安全策略要求隐藏时，不能继续展示敏感旧快照。

查询失败不能清空不相关的 mutation 状态，mutation 失败不能清空已有列表。401/403 需要立即隐藏敏感资源快照，但不构成其他并行动作 `not_applied` 的证据；owner-scoped、无正文的 exact mutation journal 必须保留到重新鉴权后对账，只有当前动作的结构化 effect 可以决定是否清除其自身记录。传输中断后的 mutation 必须进入“结果尚未确认”，先对账，不能直接显示“未保存”或“数据仍然保留”。

结果未知的同一修改目标必须保持锁定，直到该领域的权威读取证明结果，或用户在看过明确风险后主动确认新的意图。后台刷新、WebSocket invalidation、其他资源读取成功和任意页面重新渲染都不能解除该锁。外部投递只允许重试投递阶段，不能重新执行原任务。

## 7. 不同操作的可靠性强度

可靠性机制按实际副作用组合，不能全局套用同一账本：

| 操作 | 默认机制 |
| --- | --- |
| 普通查询 | 不写 operation ledger；同 scope 请求合并、有限读取重试、旧快照与权限策略 |
| 本地原子修改 | 资源 ID + revision/digest CAS + 单事务提交回执 |
| 创建或高风险命令 | 只在该领域内使用 intent digest 和紧凑幂等记录；每次重放重新鉴权 |
| 异步或外部副作用 | 领域持久阶段、CAS + lease、next attempt、第三方幂等键/回执和结果未知状态 |
| Conversation | 保留现有 client request/message、ACK、round、sequence 与重连对账合同 |

跨第三方系统不承诺普遍 exactly-once。支持幂等键时复用同一领域意图；支持结果查询时先对账；两者都不支持且响应丢失时进入人工检查，不能盲目自动重放。

## 8. 重试与实时通知

- 自动重试只适用于读取、明确幂等操作或领域持久阶段。
- 参数、权限、冲突和结果未知不自动重试。
- 429/503 遵守服务端等待时间；瞬时重试必须有上限、退避和随机抖动。
- 前端、网络库、业务服务、第三方 SDK 和 Worker 之间，每个边界只能有一个重试负责人。
- 页面卸载、scope 改变或离线时停止无意义的前台重试。
- 普通资源 WebSocket 只发送轻量 invalidation；事件必须包含足以区分 upsert、delete/tombstone、权限撤销或 scope reset 的事实。重复和乱序通知按 revision 去重，缺口后重拉权威快照。
- Conversation 继续使用自己的 durable replay 与 subscription 协议，不能被普通资源 invalidation 替代。

### 8.1 Automation 的持久恢复阶段

Automation 不把“任务跑完”和“结果送到”当作一次操作，也不用一个通用 operation 表包裹整个产品。当前只在 Automation 自己的 aggregate 中保留六个有证据的阶段：

1. **执行结束**：执行结果、terminal run 和当前 task runtime 按 exact owner/job/run 在一个事务中提交。只有这一步成功后，才允许进入外部投递。
2. **首次投递**：terminal run 的 pending 投递必须先以内部 attempt token 原子领取，再调用外部 router。两个实例同时处理时只能有一个获得 token。
3. **投递恢复**：router 明确失败或回执丢失后，结果保持为未确认的 `retrying`，后台不会自动再发。用户核对接收端后，才可用 exact owner/job/run、当前配置版本和 delivery attempts 领取新 attempt；这个动作只重投已保存的结果，不重新执行任务。
4. **运行领取**：带版本的“立即运行”和权限恢复必须把调用方确认的 `configuration_version`、权限策略修订、权限状态和 exact pending request 一直带到 durable runtime claim 的条件更新；另一实例在中途保存了新配置或推进了权限请求时，本次操作返回冲突，不得静默改用新配置、继续旧权限计划或清除新请求。页面立即运行还要用独立 `request_id + intent digest` 把 task claim 与初始 run 写入同一事务；同一身份只重放同一 run，响应丢失后的显式恢复继续使用原身份，不创建第二次运行。用户从 `denied` 手动重启时，恢复为 `ready` 与领取运行权必须是同一条条件写入。
5. **删除恢复**：删除先以 exact owner/job/configuration version 领取 deletion token，同时禁用新运行和下一次调度；之后才中断 exact 本地 attempt，收口 run、权限、投递与审计数据，最后删除任务定义。重复 DELETE 复用同一持久 token，不创建第二条删除链。
6. **主动跟进唤醒**：每次 Heartbeat wake（包括没有附加文本的唤醒）先用 owner-scoped `request_id + intent digest` 在同一配置事务栅栏内写入 durable outbox，提交后才允许派发。跨实例消费者只能领取一次；重启只恢复尚未领取的 `new`，已经开始但 claim 过期的 `processing` 必须保守收口为结果未知，不能自动再次唤醒。

跨实例 scheduler 的低频审计可以批量读取全部 `review_required` 任务的非终态 run，再按 exact owner/job 分组；禁止为每个异常任务追加一条数据库查询。

如果当前实例无法证明原物理执行已终止，删除进入 `review_required`：任务已禁用，但任务定义、运行历史和原数据仍保留；必须等 exact attempt 自然终止或管理员核对，不得因 lease 超时或进程暂时不可见就猜测已停止。删除中的迟到 terminal 只能使用 exact deletion token 走独立的 suppressed commit：保存执行结果作为停止证据，强制投递为 `not_attempted`，不能调用 router，也不能把旧 runtime 写回正在删除的任务。

这些恢复由持久 deadline、索引、合并唤醒和低频审计驱动，不做 per-task 轮询。页面对 WebSocket、focus 和 visibility 的连续刷新只做单飞行请求加一次尾随合并，不增加定时拉取。所有 token、run ID、request ID 和诊断 ID 都只用于内部精确对账，不要求用户复制或理解。

## 9. 性能与安全不变量

- FailureCore 只在失败响应序列化，不增加成功路径数据库访问。
- 普通查询默认不增加数据库写入或额外网络往返。
- 普通 CAS 修改默认保持原事务数量；审计、outbox 或多资源修改属于领域明确例外。Automation 外部投递只占用同任务分片锁并依赖数据库 exact claim，第三方延迟不得占用全局任务配置锁或阻塞其他任务。
- 不为所有 POST/PATCH/DELETE 建立全局幂等记录。
- 不保存完整旧响应、秘密、原始请求正文或可绕过新权限的缓存结果。
- 异步操作复用领域 aggregate，不重复写一份全局 operation ledger。
- 每次幂等回放、结果查询和恢复动作都重新验证当前 owner、权限和资源 scope。
- HTTP 诊断 ID、领域意图 ID 和资源/流程 ID 永不互相转换。

## 10. 当前接入状态

- HTTP：FailureCore v1、TypeScript 生成类型、私有 HTTP request context、显式 `WriteError` 和 Web 宽容解析已经存在；旧 Handler 默认不受影响。
- Loop：详情查询的 not-found 是首个只读接入样板；状态码、detail 和成功 envelope 保持不变，Loop 启动、Goal 与 Session 链路不参与该试点。
- WorkGraph：编辑器 Apply 是首个写入接入样板；只把写入前已经证明的 stale revision、无效输入和 scope 内 not-found 标记为 `not_applied`，未分类错误保持 `unknown`。它继续使用既有 `editor_id + revision`，并保留 stale revision 既有的 HTTP 422 合同，不接收、持久化或解释 HTTP 诊断 ID。
- Authentication：全局登录和桌面会话 token 在业务 Handler 之前拒绝请求时返回结构化失败；稳定 code 驱动桌面恢复，安全文案不再承担协议判断。
- Automation：任务和权限列表读取失败独立表达，权限辅助读取失败不销毁任务主快照。Create/Update/启停/删除/立即运行/审批/恢复/投递重试按各自真实阶段区分 `not_applied` 与 `unknown`；多阶段操作默认保守为 `unknown`。页面启停携带已有 `configuration_version` 做 CAS。页面创建和立即运行分别使用 Automation 自己的 `request_id + intent digest` 安全重放同一次意图；立即运行把 runtime claim 与初始 run 原子提交，刷新运行历史只按 exact `client_request_id` 证明同一次启动已受理，显式恢复复用原 ID。旧调用不携带时仍保持原有行为，且这些 ID 不得来自或转换为 HTTP `X-Request-ID`。页面副作用 journal 按 owner 和已有领域意图身份分别保存非敏感记录，持久跨越标签页和桌面 App 重启；同一 Job 的多个页面通过 Web Lock 在发送前串行，缺少该能力的非支持环境 fail closed，不用带超时的浏览器 lease 猜测互斥。storage event 只同步保护状态，不触发请求；未决副作用不得因时间经过而静默解锁。执行 terminal、结果和任务运行占用先在同一事务提交；需要投递的成功 run 先进入 durable `pending`，再以不公开的 exact attempt token 唯一领取外投。worker 可恢复尚未领取的 `pending` 和明确失败的 `failed`，但 router 已被调用后的任意错误、进程中断或完成写入失败都保持 `retrying`，只提示用户先核对，不进入自动重试或普通 CLI/API 重放。权限拒绝或任务修订取消 blocked run 时没有可投递结果，必须在同一事务把 run 收口为 `not_attempted`、清空 attempt/retry 字段，并且只有 exact 最新 terminal run 可以更新任务摘要。任务的 `last_delivery_status` 只由 exact 最新已完成 run 投影；升级时按 exact owner/job 与 `finished_at DESC, run_id DESC` 回填 succeeded/failed/cancelled 权威 run，排除 active、未完成和 skipped 行，防止历史重投覆盖当前摘要。Heartbeat wake 始终先写 durable outbox，未领取项由现有 deadline 调度恢复，已开始但结果未知的 claim 不重投。删除中的任务只允许 private deletion token 约束的 suppressed terminal 写入，强制 `not_attempted` + dead-letter 且不触发外投。投递重试不得变成重新执行任务。
- Conversation：继续使用独立 `failure_code`、client request/message identity 和 durable reconcile。
- Configuration：继续使用领域 request ID、plan digest、revision、审计与 reconcile 状态。
- 其他页面和 Handler 只有在明确接入、补齐领域证据和兼容测试后，才能使用结构化 effect 或 resolution。

任何接入都必须覆盖：旧/新 envelope、未知字段、提交前失败、提交后响应丢失、同字面值跨领域 ID、scope 切换迟到响应、权限撤销和结果未知不重放。高风险或外部操作还必须覆盖进程崩溃、lease 恢复、第三方回执丢失以及只重试失败阶段。
