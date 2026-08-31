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

### 2.1 用户可见异常合同

任何进入 Web、桌面宿主、浏览器扩展、外部 IM、CLI 或模型工具结果的用户可见异常，都必须让普通用户直接回答以下三问：

1. **Problem — 发生了什么**：哪个用户目标、对象或业务阶段没有达到预期；原因只有在已经确认且有助于恢复时才说明。
2. **Impact — 是否影响已有数据**：本次操作对已保存内容、当前输入、页面快照和后续阶段的确切影响；无法证明时必须明确说“目前无法确认”，不能用安慰性文案猜测。
3. **Recovery — 用户现在能做什么**：当前最安全的下一步、由谁执行，以及这一步会做什么。系统正在自动恢复或用户无需操作时，也必须明确说明。

`Problem / Impact / Recovery` 是产品语义槽，不是新增的 HTTP 字段、数据库表或界面固定标签。它们由现有领域事实和 `FailureCore` 共同投影为标题、正文、状态说明和动作；紧凑界面可以合并句子，但不能省略其中一项。多个字段同时校验失败时，表单级摘要只回答一次数据影响，各字段分别说明自身问题和修正方法，不需要重复相同文案。

已经被系统完全吸收、没有影响用户任务、数据、权限、可见内容或等待时间的瞬时内部故障只进入日志，不应为了“完整”而打扰用户。一旦选择展示异常，本合同全部生效。当前共享异常组件和已盘点的直接异常界面都执行本合同；任何新增界面只展示“失败”而缺少 Problem、Impact 或 Recovery，均视为产品回归。后端能提供多精确的数据影响证据仍以第 10 节为准。

### 2.2 从内部证据到用户语言

`effect` 是系统判断数据影响的证据结论，不是面向用户的状态名称。界面不得直接显示 `not_applicable`、`not_applied`、`accepted`、`committed` 或 `unknown`，也不得把它们机械映射成固定颜色、图标或严重程度。

| 内部证据 | Impact 必须表达的事实 | Recovery 的默认边界 |
| --- | --- | --- |
| `not_applicable` | 本次只是读取失败；说明当前没有最新内容、正在显示旧快照，或该区域暂时无内容。只能在证据允许时说明已保存数据没有改变 | 可重新加载、继续使用仍然可靠的旧内容，或等待系统恢复 |
| `not_applied` | 明确说明本次更改没有保存；只有界面确实保留输入时，才能说输入仍在本页 | 修正输入、权限或冲突后安全重试 |
| `accepted` | 明确说明请求已收到且仍在处理，当前不能把它当成完成 | 查看进度或等待；不得重复提交同一意图 |
| `committed` | 明确说明哪个阶段已经保存，以及失败发生在哪个后续阶段 | 只恢复失败阶段，例如刷新页面或重新投递，不得重做已完成阶段 |
| `unknown` | 明确说明目前无法确认是否生效，并指出重复操作可能造成的具体风险 | 先通过领域权威状态对账；普通“重试”不得成为主动作 |

影响说明必须使用具体对象和阶段，例如“任务已经保存，但结果还没有发送”，不能笼统写“数据不受影响”“部分成功”或“操作可能失败”。同一流程涉及执行、保存、投影和外部投递时，分别说明每个有关阶段。

### 2.3 标题、正文和动作措辞

用户界面按“标题回答 Problem，正文先回答 Impact、再回答 Recovery，动作落实 Recovery”的顺序组织。原因是可选补充，不得挤掉影响或下一步。

#### 标题

- 使用用户正在处理的对象和结果，优先写“任务列表暂时无法更新”“文件还没有上传”，不用“错误”“操作失败”“未知异常”“Error 500”这类泛化标题。
- 标题应独立可理解、尽量保持一行，不重复正文，不写解决步骤。中文标题不加句号；英文使用 sentence case，除问句外不加结尾标点。
- “失败”不是禁词，但只在它比自然结果描述更清楚时使用。面向普通用户优先写“没有保存”“无法连接”“还没有发送”；运行历史和诊断详情可以使用准确的阶段状态。
- 不从 `detail`、HTTP 状态或异常类型拼接标题。标题由稳定领域 code 和已验证上下文投影。

#### 正文

- 默认一到两句短句：第一句说明数据和当前状态，第二句说明下一步。需要解释原因时，只写已知且有助于恢复的原因。
- 使用普通、具体、非责备语言和主动语态。不要写“你忘记了”“非法”“禁止”“无效参数”“服务器内部错误”，而要说明需要什么或当前缺少什么。
- 不使用“哎呀”“糟糕”“Oops”等拟人化感叹，不用幽默弱化风险。通常不写“抱歉”“请”；只有已确认的数据丢失、服务长期不可用或用户必须联系支持等严重情况，才可简短致歉，且致歉不能代替三问。
- 不承诺无法证明的事实，例如“数据绝不会丢失”“我们已经收到反馈”“很快恢复”。只有存在真实追踪、回执或服务端等待时间时，才能说明相应事实或时长。
- 资源名、数量、时间和阶段等动态内容必须来自安全的结构化数据并本地化；秘密、路径、堆栈、内部 ID 和原始 Provider 文案不得插入正文。
- 中文使用产品中已经出现的普通名称，避免临时创造缩写；英文使用自然缩写、常见 contractions 和 sentence case。两种语言都不得逐字翻译内部枚举。

#### CTA 和辅助动作

- 主动作使用能预测结果的动词短语，例如“重新加载”“检查任务”“更新权限”“重新发送”；英文优先使用一个动词，必要时补充宾语，例如 `Reload`、`Review task`。
- 一个异常默认只有一个主动作；确有不同安全路径时最多增加一个次动作。帮助文档、复制诊断信息和关闭属于低优先级辅助动作。
- 不用“确定”“知道了”“OK”“下一步”冒充恢复动作。只有没有可执行恢复、用户只需关闭说明时，才使用“关闭”。
- CTA 必须执行与文案一致的精确领域动作，并在执行时重新鉴权。不能把未知结果的修改绑定到普通重试，也不能仅凭 `resolution.action` 自动导航或执行。

诊断信息采用渐进披露：主界面只提供“查看诊断信息”或“复制诊断信息”，展开后才显示经过脱敏的错误 code、诊断 Request ID 和支持所需上下文。诊断编号帮助支持人员关联日志，不属于用户恢复步骤。

### 2.4 组件选择与持续时间

组件由异常的作用范围、后果、复杂度和是否需要用户行动决定，不由 HTTP 状态、`category` 或 `effect` 直接决定。

| 场景 | 首选界面 | 行为合同 |
| --- | --- | --- |
| 单个输入不符合要求 | 字段旁 inline error | 与字段通过可访问描述关联，保留所有输入，直接说明如何修改；提交被拒绝时由表单摘要补充整体 Impact |
| 单个列表、卡片、面板或编辑区加载/刷新/操作失败 | 区域内 resource state 或 inline notification | 紧邻受影响区域；有旧快照时继续展示并标为不是最新；不得伪装成空状态 |
| 整页、当前页面主要任务、离线或权限状态受影响 | 页面或容器顶部 banner/message bar | 保持可见直到恢复或用户明确关闭；关闭后若阻塞条件仍在，下次进入必须再次可发现 |
| 整页没有任何可靠内容 | 页面级 error state | 替换主内容但保留导航和安全退出；不能使用“暂无内容”空状态 |
| 高后果、不可逆、必须立即决策，或离开当前流程会放大结果未知风险 | modal/alert dialog | 只承载必要信息和一到两个决策；不得因为普通加载失败就打断用户 |
| 低后果、无需行动、信息在其他位置持续可见 | toast | 不能作为异常的唯一载体；自动消失前后的同一事实必须在受影响区域、历史或通知中心可再次找到 |

Nexus 不把自动消失的 toast 作为 validation、权限拒绝、结果未知、数据风险、离线或任何需要 CTA 的异常主界面。toast 可以补充提醒，但持久界面仍须完整回答三问。窄屏不得隐藏 Impact 或 Recovery；如果空间不足，应改用可展开的持久界面，而不是只留下标题。

CLI、模型工具和外部 IM 没有视觉组件时，仍按相同顺序输出：先说发生了什么，再说数据影响，最后给出下一步。机器调用方同时依赖结构化 code/effect/action，不能解析自然语言。

### 2.5 视觉语气

- 默认使用克制、稳定、就近的企业软件画风：中性背景配一个语义色强调条或图标，不用大面积纯红、抖动、闪烁、表情符号、吉祥物或娱乐化插画。
- 严重程度必须由文字、图标和可访问语义共同表达，颜色不能成为唯一信号。红色留给已经阻塞当前任务或需要立即处理的高优先级问题；黄色用于仍可继续但需要谨慎核对的降级或风险；信息色用于系统正在恢复且不要求用户行动的状态。最终选择仍由用户后果决定，不能按 `effect` 硬编码。
- CTA 使用正常主次按钮层级；只有动作本身具有破坏性时才使用危险按钮，不能因为消息是错误状态就把“重新加载”染成危险操作。
- 同一区域只保留一个主异常面。多个问题按严重程度聚合，并提供可定位的明细，避免堆叠大量 banner、toast 或重复播报。
- 诊断详情、支持链接和技术上下文降低视觉权重，不与用户恢复动作争夺注意力。

### 2.6 无障碍合同

- 动态但不需要抢占工作的结果或进度使用可被辅助技术识别的 `status`/polite live region；只有确实需要立即注意的高优先级异常使用 `alert`/assertive。持久、非紧急说明可以使用带可访问名称的 region。不能把所有红色消息都设为 assertive。
- 非模态 inline、banner 和 toast 默认不移动键盘焦点。表单提交后有多个错误时，把焦点移到错误摘要或第一个无效字段，并确保错误链接能定位对应控件；单字段即时校验不得在用户尚未完成输入时反复打断。
- dialog 打开后进入合理的初始焦点并限制焦点在其中，关闭后回到触发控件；只有必须立即决策的异常使用 `alertdialog`。
- 视觉文本和屏幕阅读器获得的 Problem、Impact、Recovery 必须等价。图标和颜色提供冗余提示，不替代文本；字段错误通过 `aria-describedby` 等平台等价机制与控件关联。
- 需要阅读、复制或执行动作的异常不得自动消失。允许限时消失的补充 toast 必须没有必要动作，并保证同一信息可在其他持久位置找到。
- 所有动作可用键盘完成，焦点顺序可预测且不被浮层遮挡；关闭和恢复动作有清楚的可访问名称。页面缩放和窄屏重排时不得截断三问。
- 满足 WCAG 2.2 AA 的文本与非文本对比度、可见焦点和状态消息要求；遵循减少动态效果偏好。相同故障在状态未变化时只播报一次，避免 live region 风暴。

### 2.7 推荐文案模式

以下是语义模式，不是可脱离证据复制的万能文案：

| 场景 | Problem | Impact | Recovery / CTA |
| --- | --- | --- | --- |
| 后台刷新失败但有快照 | 任务列表暂时无法更新 | 当前仍显示上次加载的内容，已保存的任务没有改变 | 重新加载后可查看最新状态 / **重新加载** |
| 表单提交前校验失败 | 任务名称需要修改 | 任务还没有保存，已填写内容保留在本页 | 输入 1–50 个字符的名称后再次保存 / **保存** |
| 修改请求结果未知 | 还无法确认任务是否已创建 | 网络中断前请求可能已经提交，再次创建可能产生重复任务 | 先检查任务列表 / **检查任务** |
| 已保存但后续投递失败 | 任务已完成，但结果还没有发送 | 运行结果已保存在历史中，任务不会重新运行 | 核对接收端后只重新发送结果 / **查看运行** |
| 前置权限拒绝 | 当前账号不能修改这个 Room | 这次更改没有保存，Room 现有内容没有改变 | 联系 Room 管理员获取编辑权限 / **查看权限** |

英文遵循同一信息顺序。例如：标题 `Task list isn’t up to date`；正文 `You’re still seeing the last loaded list. Saved tasks haven’t changed.`；动作 `Reload`。不得把中文内部状态直译为 `Committed`、`Not applied` 或 `Unknown effect`。

### 2.8 用户可见异常验收

一个异常只有同时满足以下条件才算完成接入：

1. 仅看当前界面截图或辅助技术可访问树，普通用户可以回答 Problem、Impact 和 Recovery；日志、开发者工具和代码注释不算答案。
2. Impact 与服务端事务、revision、durable ACK、领域回执或第三方权威回执一致；没有证据时明确使用结果未知表达。
3. Recovery 是当前身份和阶段下安全、可执行的动作；未知副作用不出现普通重试，不会重做已完成阶段。
4. 错误紧邻正确作用范围，不替换成空状态，不清空仍可靠的快照或未提交输入，也不污染其他 Session、Room、Agent 或 owner。
5. 标题和正文没有内部术语、原始异常、秘密、路径或需要用户理解的 ID；诊断信息经过脱敏并渐进披露。
6. 组件、持续时间和严重程度与实际后果匹配；需要行动的消息保持可发现，窄屏不丢正文。
7. 键盘、焦点、屏幕阅读器、对比度、缩放、减少动画和本地化均通过对应回归。
8. HTTP、WebSocket、Runtime、MCP、CLI、桌面宿主、浏览器扩展和 IM 中的同一领域失败保持同一三问语义，即使外观不同。

验收不能只测试服务端明确返回错误，还必须覆盖提交前失败、提交后响应丢失、离线、超时、权限变化、scope 切换、进程中断和第三方回执丢失。任何无法说明数据影响的 mutation 异常默认按结果未知验收。

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
- 对空 code/category/effect 做安全兜底；code 和 resolution action 只接受有长度上限的 `domain.reason` 语义名，非法值不得进入 wire；
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
- 用户响应不包含堆栈、SQL、文件路径、Provider 原始错误或秘密；共享失败 writer 的日志只保留结构化 code、阶段、effect、HTTP 诊断 ID、cause 是否存在及其类型。领域只有在自己的安全边界完成脱敏后，才能另行记录内部 cause 细节。
- 日志同样不是秘密仓库：访问令牌、密码、连接密钥、完整请求正文和可直接复用的会话身份不得记录；所有外部文本进入日志前必须经过结构化字段约束或清理。

## 10. 当前接入状态

- HTTP：FailureCore v1、TypeScript 生成类型、私有 HTTP request context、显式 `WriteError` 和 Web 宽容解析已经存在；旧 Handler 默认不受影响。
- Loop：详情查询的 not-found 是首个只读接入样板；状态码、detail 和成功 envelope 保持不变，Loop 启动、Goal 与 Session 链路不参与该试点。
- WorkGraph：编辑器 Apply 是首个写入接入样板；JSON 解析在调用业务服务前失败时也输出同一 FailureCore。只把写入前已经证明的 stale revision、无效输入和 scope 内 not-found 标记为 `not_applied`，未分类错误保持 `unknown`。它继续使用既有 `editor_id + revision`，并保留 stale revision 既有的 HTTP 422 合同，不接收、持久化或解释 HTTP 诊断 ID。
- Memory：workspace 文件读取在成功响应中增加由正文计算的稳定 `revision`，写入可选接收 `expected_revision`；旧调用不携带时保持原无条件语义，Memory 编辑器必须携带读取基线。已过期基线在落盘前返回 `workspace.file_revision_conflict + not_applied`，保留服务器正文和用户草稿。页面先加载 exact Agent + path 最新版本并对照，只有用户明确选择覆盖时才以该最新 revision 再次提交；断线或响应丢失会锁定保存并先读回对账，不盲目重放。该 revision 不是 Agent、Session、request 或幂等身份，不入库且不需要数据迁移。
- Workspace 文本编辑器：只有 exact owner generation + Agent + path 的首次 GET 成功并取得正文 `revision` 后才开放编辑，每次 PUT 都携带该读取基线。实时外部更新只能在草稿干净时投影；dirty 或正在编辑的草稿保持原样并进入冲突选择。`workspace.file_revision_conflict + not_applied` 先加载最新正文和 revision，随后只接受用户明确选择放弃草稿或用最新 revision 覆盖；transport/legacy `unknown` 锁定当前页保存，只用 exact 文件 GET 对账：正文等于已提交草稿只证明当前意图已经满足，revision 仍等于提交基线只证明当前状态允许用户明确再次保存，两者皆否则进入冲突选择。内容 revision 不能证明请求是否曾短暂生效，页面不得作此宣称，也不自动重放 PUT。owner、Agent、path、401/403 变化清除旧 scope，其他读取失败保留已成功加载的同 scope 内容和草稿。该接入复用现有内容 revision，不新增 ID、持久账本或迁移。
- Workspace 文件列表：create/rename/delete/upload 在业务调用前的请求格式、Agent/条目不存在和 typed 参数拒绝返回 `not_applied`；服务阶段无法证明提交边界的错误返回 `unknown`。成功修改与随后列表刷新是两个阶段，刷新失败只恢复列表，不得把已完成修改误报成失败或再次发送。transport/legacy/accepted/unknown 按 exact Agent + command + source/target path 锁住当前页同一意图，create/rename/delete 只用该 Agent 的权威文件列表判断当前目标状态是否已经出现；对账本身不调用修改接口。多文件上传按浏览器文件 identity 顺序保存已完成、未确认、明确未完成和未开始项，遇到未知项即停止后续上传。上传端可能按内容去重或自动改名，而当前列表不提供内容摘要或请求回执，因此未知上传即使刷新成功也不能仅凭文件名确认，只能由用户核对后显式允许一条新意图；该内存锁不跨页面刷新。当前接入不新增业务 ID、全局幂等表或迁移。
- Authentication：全局登录和桌面会话 token 在业务 Handler 之前拒绝请求时返回结构化失败；稳定 code 驱动桌面恢复，安全文案不再承担协议判断。
- Agent 创建：Web 创建入口先生成可选的 owner-scoped `creation_request_id`，并在发送任何副作用前把非敏感的 request ID 与 `pending/unconfirmed` 状态写入该 owner 的本地 journal；不保存名称、表单、prompt、token、请求正文、intent digest 或 HTTP 诊断 ID。journal 的读取、写入或跨标签页 Web Lock 不可用时 fail closed；关闭弹窗、重载 App、普通 Agent 目录刷新和时间经过都不能清除未决创建。页面重试前必须先按 exact request ID 查询权威回执；只有用户再次明确提交同一创建意图时才复用该 ID，后台和普通读取不会自动重放创建。后端只在请求携带该字段时启用领域幂等，旧调用缺省时完全保留原创建语义且不增加 receipt 写入；同 owner + request ID 的首次请求先在 Agent 领域表保留服务端生成的原 Agent ID/workspace 路径和规范化 intent 的 SHA-256 摘要，其中业务标签必须先按实际落库规则去空、去重并保留规范显示值，不保存完整请求或秘密。同 ID + 同 digest 只能投影同一 Agent，同 ID + 不同 digest 稳定冲突，跨 owner 互不相认。回执以 `reserved -> workspace_prepared` 记录可恢复文件阶段，lease 过期本身从不把 pending 推断为 `not_applied`、不生成新 Agent 身份，也不触发后台恢复；显式重放只能在读取 exact pending receipt 后以同一 Agent ID/path 和 claim fence 继续幂等准备，旧 claim 不能提交。Agent、Profile、Runtime 与 `committed` receipt 必须同事务提交；提交结果不确定时只读 exact receipt，绝不清理可能已经属于成功 Agent 的 workspace。只有准备阶段明确失败且 `failed` 墓碑已经提交后，才可清理该 exact 未提交 workspace。Agent 删除事务同时把已提交 receipt 改为 `deleted`；迟到重放只能返回删除事实，不能再次创建。每次携带业务 ID 的创建最多增加一条小型 receipt，`committed/failed/deleted` 终态不按时间清理，因为时间型 GC 会重新打开迟到重放；容量增长只与显式幂等创建次数一致，并由 owner/request 主键和 owner/Agent 索引保持 exact 查询。GET `/agents/create-requests/{creation_request_id}` 只做 exact owner 对账，不执行创建；`pending` 是“已受理、尚未有终态”，不能因 lease 或超时伪装成失败。
- Automation：任务和权限列表读取失败独立表达，权限辅助读取失败不销毁任务主快照。Create/Update/启停/删除/立即运行/审批/恢复/投递重试按各自真实阶段区分 `not_applied` 与 `unknown`；多阶段操作默认保守为 `unknown`。页面启停携带已有 `configuration_version` 做 CAS。页面创建和立即运行分别使用 Automation 自己的 `request_id + intent digest` 安全重放同一次意图；立即运行把 runtime claim 与初始 run 原子提交，刷新运行历史只按 exact `client_request_id` 证明同一次启动已受理，显式恢复复用原 ID。旧调用不携带时仍保持原有行为，且这些 ID 不得来自或转换为 HTTP `X-Request-ID`。页面副作用 journal 按 owner 和已有领域意图身份分别保存非敏感记录，持久跨越标签页和桌面 App 重启；同一 Job 的多个页面通过 Web Lock 在发送前串行，缺少该能力的非支持环境 fail closed，不用带超时的浏览器 lease 猜测互斥。storage event 只同步保护状态，不触发请求；未决副作用不得因时间经过而静默解锁。执行 terminal、结果和任务运行占用先在同一事务提交；需要投递的成功 run 先进入 durable `pending`，再以不公开的 exact attempt token 唯一领取外投。worker 可恢复尚未领取的 `pending` 和明确失败的 `failed`，但 router 已被调用后的任意错误、进程中断或完成写入失败都保持 `retrying`，只提示用户先核对，不进入自动重试或普通 CLI/API 重放。权限拒绝或任务修订取消 blocked run 时没有可投递结果，必须在同一事务把 run 收口为 `not_attempted`、清空 attempt/retry 字段，并且只有 exact 最新 terminal run 可以更新任务摘要。任务的 `last_delivery_status` 只由 exact 最新已完成 run 投影；升级时按 exact owner/job 与 `finished_at DESC, run_id DESC` 回填 succeeded/failed/cancelled 权威 run，排除 active、未完成和 skipped 行，防止历史重投覆盖当前摘要。Heartbeat wake 始终先写 durable outbox，未领取项由现有 deadline 调度恢复，已开始但结果未知的 claim 不重投。删除中的任务只允许 private deletion token 约束的 suppressed terminal 写入，强制 `not_attempted` + dead-letter 且不触发外投。投递重试不得变成重新执行任务。
- Goal（Web 生命周期）：当前 clear/pause/resume/update 页面按 exact owner generation、Session 和 Goal 意图串行化。只有服务端明确返回 `not_applied` 才允许用户直接重试；transport、legacy 或其他 `unknown` 只通过当前 owner scope 的 Goal 与 binding 只读接口核对，未变化、并发 version 推进或 objective rewrite 均不能证明原修改未发生。成功写入与后续刷新失败分别表达；主读取失败保留同 scope 快照但禁止继续修改，binding 辅助读取失败不销毁 Goal；401/403、owner 或 Session scope 变化清除旧快照和当前页锁；页面不自动重发副作用。**Remaining gap**：Goal lifecycle Handler 仍使用旧 `WriteFailure`，当前 wire/read model 没有把一次 update/pause/resume/clear 与 durable result 精确关联的领域 mutation identity、CAS 或 receipt；objective update 还允许服务端 rewrite。因此 unknown 可能在当前页面长期保持，内存锁也不跨页面刷新或 Session 切换，现阶段不能宣称持久防重或完全对账。补齐前不得用 unchanged GET、HTTP `X-Request-ID`、正文、时间或 version 邻近猜测结果；未来修复必须属于 Goal 自身协议并保持 owner/Session/Goal scope，不得建立全局 ledger。
- Conversation：继续使用独立 `failure_code`、client request/message identity 和 durable reconcile。
- Configuration：继续使用领域 request ID、plan digest、revision、审计与 reconcile 状态。
- 尚未显式接入 `FailureCore` 的旧 Handler 可以继续返回兼容 envelope，但前端仍必须结合读取/修改语义保守回答三问：读取失败不得伪装成空状态，修改结果缺少领域证据时一律按“目前无法确认”处理并禁止普通重试。只有补齐领域证据和兼容测试后，页面才能把结果进一步说明为 `not_applied`、`accepted` 或 `committed`，也只有领域规则可以给出结构化 resolution。

任何接入都必须覆盖：旧/新 envelope、未知字段、提交前失败、提交后响应丢失、同字面值跨领域 ID、scope 切换迟到响应、权限撤销和结果未知不重放。高风险或外部操作还必须覆盖进程崩溃、lease 恢复、第三方回执丢失以及只重试失败阶段。

## 11. 标准依据与 Nexus 取舍

本节记录用户可见异常合同的公开依据和产品取舍。RFC 9457 与 Google AIP 是传输和机器语义规范，不是界面组件规范；`FailureCore v1` 继续使用 Nexus 现有 envelope，不宣称采用 `application/problem+json`。各设计系统的组件名称和视觉实现也不直接复制到 Nexus。

### 11.1 跨标准共识

公开规范和成熟设计系统在以下原则上高度一致：

- **机器语义与用户文案分开**：稳定 code、type/reason 和结构化 metadata 供程序判断；本地化文案可以改进，调用方不得解析自然语言。
- **具体、简短、可行动**：标题说明用户目标发生了什么，正文补充必要上下文和解决办法，技术细节放入渐进披露或支持信息。
- **按上下文就近展示**：字段问题靠近字段，区域问题留在区域，页面或系统状态使用持久 banner，只有必须立即决策时才打断用户。
- **保留用户工作**：校验失败保留输入，刷新失败保留仍可靠的快照，局部组件故障不拖垮整页。
- **不要责备或虚构**：使用普通语言，不暴露机器错误，不猜原因、不作无法兑现的恢复承诺。
- **恢复动作必须匹配风险**：按钮明确说明动作；自动重试只用于不会造成意外状态变化的请求，未知副作用先对账。
- **信息不能只靠颜色或视觉位置**：状态需要可访问名称、合理 live region、键盘路径和焦点管理；需要行动的信息不能在用户来不及处理时消失。

“每一种用户可见异常都明确说明数据影响”是 Nexus 在这些共识上的增强要求。Atlassian 和 Shopify 明确强调后果/影响，Adobe 强调结果、原因和修复；Nexus 进一步把 Impact 绑定到事务和领域回执，因为 Agent、Automation、Conversation 和外部投递可能跨越多个持久阶段，普通“失败”无法安全指导重试。

### 11.2 分歧与决策

| 公开实践中的分歧 | Nexus 决策 |
| --- | --- |
| RFC 9457 的 `title` 对同一 problem type 应稳定；界面设计系统要求标题具体描述本次情境 | 协议 code 保持稳定，用户标题由 code + 安全结构化上下文本地化生成；不把协议 `detail` 当标题解析 |
| SAP message box 允许标准“Error”标题；Apple、Fluent、Carbon、Atlassian 和 Shopify 更强调信息型标题 | 使用“任务列表暂时无法更新”这类具体标题，不使用孤立的“错误”或错误码 |
| SAP 不将 toast 用于错误；Fluent、Carbon、Adobe 和 Salesforce 在限定场景允许错误 toast，Salesforce 默认让 error/warning toast 保持到关闭 | toast 永不成为异常唯一载体；需要行动、结果未知或有数据风险时使用持久 inline/banner/dialog。补充 toast 自动消失后，信息仍可在原位置找到 |
| 部分企业系统广泛使用错误 dialog；Apple、Fluent、Spectrum 强调少打断 | 仅在高后果、必须立即决策或离开会放大风险时使用 dialog；普通加载和保存错误就近显示 |
| GOV.UK、Atlassian 和 Cloudscape 通常避免 “please/sorry”；Microsoft 和 Adobe 允许在非用户责任或严重损失时有限使用 | 默认不写“请/抱歉”；确认数据丢失、长期中断或必须升级支持时可简短致歉，但三问仍优先 |
| Fluent 对 error/warning 常用 assertive；Salesforce toast 使用 polite；USWDS 按紧迫程度选择 alert/status/region | ARIA 强度由是否需要立即注意决定，不由红色或 error 名称决定；assertive 只用于真正紧急情况 |
| GOV.UK 默认提交后校验；Cloudscape 允许完成字段输入后的 on-blur 校验 | 首次默认在提交或进入下一步时校验；只有能减少返工且不会干扰慢速输入时才增加 on-blur/输入中反馈 |
| 各系统对红色覆盖范围不同，Shopify 将红色收窄到需要立即处理的问题 | Nexus 使用低面积语义色；红色表示当前任务被阻塞或需要立即处理，黄色表示可继续但需要核对。颜色不代表数据是否提交 |

### 11.3 官方参考

- [RFC 9457 — Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457.html)：稳定 problem type、简短 title、针对本次发生情况的 detail、扩展字段和安全披露；detail 不能被客户端解析为协议。
- [RFC 9110 — HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110.html#name-if-match) 与 [Google AIP-154 — Resource freshness validation](https://google.aip.dev/154)：`If-Match`/资源 ETag 在执行写方法前校验客户端看到的版本，用来防止多个客户端并行修改造成 lost update；前置条件不成立时不得执行写入。版本属于资源，不属于一次 HTTP 传输。
- [Google AIP-193 — Errors](https://google.aip.dev/193) 与 [AIP-194 — Automatic retry configuration](https://google.aip.dev/194)：机器可读 `ErrorInfo`、本地化且可行动的消息、结构化上下文，以及只有不会造成意外状态变化的请求才自动重试。
- [AWS Builders’ Library — Making retries safe with idempotent APIs](https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-APIs/)：重试身份必须绑定同一业务意图，迟到请求和“同一 ID、不同意图”都要由领域协议处理，不能把传输 Request ID 当幂等依据。
- [Stripe — Idempotent requests](https://docs.stripe.com/api/idempotent_requests) 与 [Advanced error handling](https://docs.stripe.com/error-low-level)：相同幂等键只能绑定相同参数，网络错误后的安全重试依赖服务端保存的同一操作结果；无法判断服务端结果时应按 indeterminate 处理，不能把连接错误当作未执行证据。Nexus 只在确有重复副作用风险的领域采用同类机制，不为全部写请求建立全局账本。
- [GitHub REST API — Best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)：优先事件通知、避免无界轮询和并发请求，读取使用条件请求，限流遵守 `Retry-After` 并采用有上限的退避；这支持 Nexus 的轻量 invalidation、读取单飞行和单一重试负责人取舍。
- [Azure Architecture Center — Compensating Transaction](https://learn.microsoft.com/en-us/azure/architecture/patterns/compensating-transaction)：补偿是可失败、需要记录进度的领域操作，不等同于数据库回滚；并发发生后恢复旧值可能覆盖他人修改，高影响或不明确场景应停下并交给人工核对。
- [Azure Architecture Center — Retry pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/retry)：只有已识别为瞬时且重放安全的操作才重试；次数和等待必须有界，非幂等写入可能已完成但响应丢失，重复发送会造成二次副作用。重试策略应由理解完整业务结果的一层统一负责，避免多层叠加。
- [Microsoft Fluent 2 — Message bar](https://fluent2.microsoft.design/components/web/react/core/messagebar/usage)、[Toast](https://fluent2.microsoft.design/components/web/react/core/toast/usage) 和 [Dialog](https://fluent2.microsoft.design/components/web/react/core/dialog/usage)：按范围和紧迫度选择界面，标题具体，正文一到两句，错误和警告提供明确动作。
- [Microsoft Business Central — User experience guidelines for errors](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/developer/devenv-error-handling-guidelines) 与 [Actionable errors](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/developer/devenv-actionable-errors)：What went wrong / How to fix / Clear action，Fix-it、Show-it 和分离的诊断详情。
- [Microsoft Windows — Writing style](https://learn.microsoft.com/en-us/windows/apps/design/style/writing-style) 与 [Error messages](https://learn.microsoft.com/en-us/windows/win32/uxguide/mess-error)：从用户目标描述问题，语言简短、具体、非责备，并在可能时给出解决办法；保留可修正输入，使用能完成任务的最轻量界面。Nexus 另外强制补充经证据确认的数据影响。
- [AWS Cloudscape — Errors](https://cloudscape.design/patterns/general/errors/) 与 [Error messages](https://cloudscape.design/patterns/general/errors/error-messages/)：上下文化错误、局部 error boundary、可行动普通语言，以及原始错误只在可展开详情中出现。
- [IBM Carbon — Notification](https://carbondesignsystem.com/components/notification/usage/)：错误正文必须包含 user action，inline 异常保持可见，actionable notification 限制动作数量，并为 toast 提供再次访问路径。
- [Apple Human Interface Guidelines — Alerts](https://developer.apple.com/design/human-interface-guidelines/alerts) 与 [Feedback](https://developer.apple.com/design/human-interface-guidelines/feedback)：alert 只用于关键且最好可行动的信息，标题不能只是“Error”，普通连接问题优先保留内容并非侵入式提示。
- [SAP Fiori — Message Handling](https://experience.sap.com/fiori-design-web/messaging/)、[Message Toast](https://experience.sap.com/fiori-design-web/message-toast/) 与 [UI Text Guidelines](https://experience.sap.com/fiori-design-web/ui-text-guidelines-for-sap-fiori/)：用普通语言精确说明问题并建议建设性方案，toast 主要用于简短成功反馈，错误不应自动消失。
- [Atlassian Design — Error messages](https://atlassian.design/foundations/content/designing-messages/error-messages)：标题可扫描，正文说明问题、后果和下一步，不知道原因时不能编造，CTA 使用明确动词。
- [GOV.UK Design System — Error message](https://design-system.service.gov.uk/components/error-message/)、[Error summary](https://design-system.service.gov.uk/components/error-summary/) 与 [Recover from validation errors](https://design-system.service.gov.uk/patterns/validation/)：字段错误与摘要同时存在、保留所有输入、使用正向具体措辞并把焦点带到可修正位置。
- [WCAG 2.2 — Error Identification](https://www.w3.org/WAI/WCAG22/Understanding/error-identification)、[Error Suggestion](https://www.w3.org/WAI/WCAG22/Understanding/error-suggestion)、[Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages) 与 [Timing Adjustable](https://www.w3.org/WAI/WCAG22/Understanding/timing-adjustable)：文本识别和修正建议、可编程状态播报，以及限时消息必须有持久替代路径。
- [Adobe Spectrum — Writing for errors](https://spectrum.adobe.com/page/writing-for-errors/)：What happened / cause if known / how to fix，并按 consequence、complication 和 action 选择 inline、banner、dialog 或 toast。
- [Shopify Polaris — Error messages](https://polaris-react.shopify.com/content/error-messages)：标题说明错误对用户的影响，正文说明修复方法，CTA 提供一步解决路径，红色留给需要立即处理的问题。
- [Salesforce Lightning — Toast](https://developer.salesforce.com/docs/platform/lightning-component-reference/guide/lightning-toast)：error/warning 默认 sticky、语义 live region 和移动端内容收缩，说明重要恢复信息不能只放在可能隐藏的 toast 正文。
- [U.S. Web Design System — Alert](https://designsystem.digital.gov/components/alert/)：只有需要立即注意时使用 `role="alert"`，较低紧迫度分别使用 `status` 或有名称的 `region`。
- [OWASP — Error Handling Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Error_Handling_Cheat_Sheet.html) 与 [Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)：用户响应不得泄露实现细节，内部原因进入受保护的结构化日志；日志必须防注入并排除访问令牌、密码、连接密钥和其他敏感数据。
