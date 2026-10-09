# Nexus 错误提醒与恢复规范

> 状态：当前规范。范围：用户可见错误、HTTP 失败事实和安全恢复边界。

## 1. 目标

错误机制只做三件事：

1. 告诉用户哪个目标没有完成；
2. 用一句话告诉用户现在能做什么；
3. 在确有安全动作时提供一个直接动作。

- 不影响用户任务、数据、权限、可见内容或等待时间的内部故障只写日志，不弹提示。
- 错误判断是确定性产品逻辑，不调用 LLM，也不让模型生成恢复动作。
- 不统一各领域的事务协议。Agent、Automation、Conversation、Configuration、Goal、Execution 和 Workspace 各自拥有 request ID、revision、receipt、round 或 run 身份（见 §7）。

## 2. 用户界面

### 2.1 默认结构

- 第一行：具体标题，例如“任务没有保存”；不用“发生错误”或错误码。
- 第二行：一句短说明，例如“请稍后重试”或“请修改名称后重新保存”。不要求单独补齐“影响”和“下一步”。
- 可选动作：至多一个主动作，只用“重试”“刷新”“重新登录”等短标签。

动作规则：

- 当前界面已有发送、保存、刷新或权限选择入口时，不重复渲染同义按钮；只有错误组件能直接、安全地完成恢复时才显示动作。
- 例外：结果未知、重复发送、覆盖或删除风险，必须用一句话指出具体风险，并提供“刷新当前页面”“打开运行记录”等明确动作；不能用“核对状态”这类抽象文案。
- 在两个互斥业务结果中选择（例如采用最新版本或覆盖它）属于独立 decision surface，不是 error recovery。它可提供两个明确选择，但 error 态类型不得借此恢复第二动作。

展示规则：

- 主提示不展示堆栈、SQL、文件路径、Provider 原文、HTTP 状态、内部 ID 或服务端 `detail`。诊断号只出现在折叠诊断或复制诊断信息中。
- 表单字段错误留在字段附近并保留输入。页面读取失败、区域失败和修改失败分别留在所属区域，不用全局 toast 代替。
- 需要用户处理、权限变化或结果未知的提示不能自动消失。
- 同一区域有多个状态时只展示最需要处理的一条。Conversation Composer 固定按会话可靠性、会话索引读取、Provider 配置、Goal 的顺序取首项；较低优先级状态保留在领域状态中，前一项解除后再显示。
- 共享错误组件不接受不会显示的备用正文，也不为满足模板生成恢复说明。
- 重复失败不增加更多解释。桌面端可在设置中导出日志作为诊断材料。产品没有通用问题提交接口，错误面不显示“报告问题”按钮。

> 非当前规范：真实的“上报问题”入口属于未来规划；接入后应作为独立帮助能力复用，而不是复制到每条错误里。

### 2.2 常见场景

| 场景 | 标题 | 一句说明 | 动作 |
| --- | --- | --- | --- |
| 输入无效 | 哪个内容需要修改 | 请修改具体字段 | 通常无，复用原提交入口 |
| 读取失败 | 哪个内容未加载 | 请稍后重试 | 没有现成入口时显示“重试” |
| 内容可能过期 | 哪个内容未更新 | 请刷新 | “刷新” |
| 修改明确未提交 | 哪个修改未保存 | 请重试 | 复用保存入口或显示“重试” |
| 修改结果未知 | 哪个结果未确认 | 请勿重复操作 | “刷新”或打开具体记录 |
| 登录或权限失效 | 当前账号无法访问 | 请重新登录或检查权限 | 有直接能力时提供 |
| 消息投递未知 | 消息状态未确认 | 刷新对话，确认前不要重发 | “刷新” |

动作必须与文案一致，并在执行时重新鉴权。未知副作用不能提供普通“重试”按钮。

## 3. 最小失败协议

普通 HTTP 失败可在现有 envelope 中附加 `FailureCore v1`：

```json
{
  "code": "409",
  "message": "failed",
  "data": {
    "detail": "兼容旧客户端的安全说明",
    "failure": {
      "version": 1,
      "code": "automation.configuration_conflict",
      "category": "conflict",
      "effect": "not_applied",
      "transport_request_id": "optional-diagnostic-id"
    }
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `version` | 固定为 `1` |
| `code` | 稳定的 `domain.reason` 机器分类 |
| `category` | 开放分类，例如 validation、authentication、authorization、conflict、transport、internal |
| `effect` | `not_applicable`、`not_applied`、`accepted`、`committed` 或 `unknown` |
| `transport_request_id` | 可选诊断关联号，仅用于排障 |

- `FailureCore` 只有上述五个字段，不包含用户文案、恢复动作、URL、命令或重试等待时间。
- 限流/排队等领域的等待信息使用所属协议；Conversation 历史索引的 `retry_after_ms` 属于该读取协议，不属于 FailureCore。
- `transport_request_id` 只能复用经过限长和字符校验的当前 `X-Request-ID`。它不能成为幂等键、授权、路由、缓存键或业务身份，也不能替代任何领域 request ID。

## 4. 后端边界

- 旧 Handler 可继续使用兼容 `WriteFailure`，默认行为不变；接入结构化失败的 Handler 显式调用 `WriteError`。
- Handler 只把已经由事务、revision、receipt 或领域状态证明的影响写入 `effect`；无法证明时使用 `unknown`。
- 业务错误用 typed/sentinel error 或明确结果映射；禁止根据 `err.Error()` 文案猜 code、category 或 effect。
- `detail` 必须安全，但对 v1 客户端只是旧协议兼容内容，不参与界面决策。
- 共享 writer 记录 code、阶段、effect、诊断号和 cause 类型；不记录秘密、完整请求正文或外部原始错误。
- `FailureCore` 只在失败响应序列化，不增加成功路径数据库访问，也不建立全局 operation ledger。

## 5. 前端边界

### 5.1 解析与文案

- HTTP 层宽容解析旧 envelope 和 `FailureCore v1`；未知版本或字段不使响应解析失败。
- 结构化失败的用户正文始终使用当前界面的本地化文案，不直接显示服务端 `detail`。
- 共享层只投影 access、code、category 和 effect，不推测领域恢复动作。
- 业务界面根据当前目标选择标题、说明和至多一个动作。

### 5.2 读取

- 首次读取失败显示错误，不伪装成空状态。
- 刷新失败保留仍可信的同 scope 快照，并说明内容可能不是最新。
- 读取辅助数据失败不能摧毁仍有权限的主快照。
- 401/403 立即隐藏该 scope 的敏感快照和交互层。
- scope、owner、resource identity 和请求代次变化后，旧响应不得写回新页面。

### 5.3 修改

- 当前页面可以防重复点击，并在结果未知时暂时锁住同一动作。
- 浏览器不保存通用 mutation journal，不用 Web Locks 充当服务端并发控制，也不因本地存储不可用拒绝业务操作。
- 页面刷新、多窗口和应用重启后直接读取服务端权威状态（revision/receipt）；前端影子状态不是真相源。
- 客户端不自动重放创建、修改、删除、运行、投递或工具调用。
- 只有事务、领域 receipt、revision/CAS、durable ACK 或权威读取能证明结果；HTTP 成功/失败、时间经过或诊断号本身不能证明数据影响。断线/超时进入 `unknown` 并先对账。

密码修改：

- 使用 user-scoped exact `request_id`，终态回执为 `committed|not_applied`；回执只保存 request identity、终态与收口时间。
- commit 事务必须先领取该 request 的 committed 终态，再对已验证的旧 hash 做 CAS，两者一起提交，使两个窗口不能用同一旧密码互相覆盖。
- `not_applied` 回执与 committed 领取互斥，必须阻止同 request 的迟到写入。
- 浏览器只可持久不含秘密的 request 指针，不得持久密码草稿。
- unknown 时不得创建 fresh request。页面仍保有原草稿时，用户可显式续行同一 exact request；草稿已丢失时只能显式放弃，由服务端原子写入 `not_applied` 后解锁。
- receipt 不存在本身不证明并发请求不会提交。

Subscription Admin：

- 未对账 mutation 锁与可见 feedback 是两个独立状态；dismiss、读取失败或其他提示替换都不得解锁。
- 服务端只在写入调用前失败时返回 `not_applied`；写入调用报错按 `unknown`；写入成功但回读失败返回 `committed`。

## 6. 重试

- 普通读取可以有限重试；参数、权限、冲突和结果未知不自动重试。
- 明确幂等的领域阶段可以复用同一业务 request ID；每次仍重新鉴权。
- 前端、服务、第三方 SDK 和 worker 之间，一个阶段只能有一个重试负责人。
- 第三方不支持幂等键或结果查询时，响应丢失进入人工核对，不能假装 exactly-once。

## 7. 领域安全边界

各领域身份继续各自拥有真相：Agent 创建的 owner-scoped `creation_request_id`、密码修改的 user-scoped `request_id`、Conversation `client_request_id/client_message_id`、Configuration/Automation/Goal/Execution 既有的 `request_id` 与 run/round/resource identity。

### Agent 创建

- `creation_request_id`、intent digest 和创建 receipt 由 Agent 领域持有。
- 回执只保存 intent digest、保留的 Agent identity/path 和阶段，不保存完整请求或秘密。
- Agent/Profile/Runtime 与 committed receipt 同事务；删除与 receipt 墓碑同事务。
- 浏览器可保存非敏感 request ID 以便重载后查询，但存储失败不阻止创建。
- 未知结果只查询 exact receipt，不清理可能已提交的 workspace。
- lease 到期不把 pending 猜成 `not_applied`，也不触发后台重放。

### Automation

服务端真相：`configuration_version`、创建/立即运行 request ID、run、delivery attempt、deletion token 和 heartbeat outbox。

运行领取、执行结束、首次投递、人工重投、任务删除和 Heartbeat wake 是六个独立的持久阶段：

| 阶段 | 规则 |
| --- | --- |
| 人工运行 | 先按 exact owner/job/configuration/permission/request identity 把 runtime claim 与初始 run 单事务提交，之后才 dispatch |
| 执行结束 | terminal run 与 task runtime 先按 exact owner/job/run 单事务提交，之后才能以内部 attempt token 领取外投 |
| 投递结果未知 | router 已调用但结果未知时保持 `retrying`；后台永不自动补投 |
| 人工重投 | 仅在用户核对接收端后，用配置版本和 delivery attempts 显式领取新 attempt |
| 任务删除 | 先持久 claim、禁用新运行并推进配置版本；再中断 exact 本地 attempt，原子收口 run/权限/投递/审计并删除定义 |
| 删除未证实停止 | 无法证明原执行实例已停止时进入 `review_required`，保留任务与历史，禁止按超时猜测 |
| 删除中迟到 terminal | 只能走 exact deletion token 的独立 suppressed commit，强制 `not_attempted`，不得投递或改写 task runtime |
| Heartbeat wake | 在配置事务栅栏内先写 durable outbox；未领取项可恢复；已开始但结果未知的 claim 不得自动重投 |

- 查询不得隐式写策略或调度状态。
- deadline 由索引、合并唤醒和低频审计驱动，不做 per-task 轮询。
- Web 只保留当前页面的 pending/unconfirmed 状态；重新打开页面直接读取任务、运行历史和权限状态。创建任务的 request ID 可作为 receipt 查询辅助，但不是提交门槛。

### Workspace 与 Memory

- 文件 revision 保护并发编辑。冲突保留用户草稿；覆盖必须基于最新 revision 并由用户明确选择。
- 上传结果未知且无法由内容摘要或 receipt 证明时停止后续上传，不按文件名猜成功。

### Conversation

- 使用独立的 client request/message、Session、round、Agent round、failure code、ACK 和重连对账协议。
- 首次 WebSocket error 不是终态；客户端不自动重发 prompt、工具或副作用命令。
- 新一次 `submission_started` 不能证明旧 `delivery_unknown`、Provider retry 或连接故障已恢复；只有与原身份匹配的 ACK、round 进展或权威 Session 对账才能解除。
- Provider/runtime 的终态错误原文不得伪装成 assistant 正常回复，只投影为结构化可靠性提示。该提示只出现在 Composer 状态栈，不写入 Feed、历史、未读或滚动几何。

### Goal 与 Configuration

- 使用各自的 revision、request ID、plan digest、审计或回执。
- 缺少领域身份时，结果未知只能在当前页锁定和对账，不能用 HTTP 诊断号补造身份。

## 8. 验证原则

验证围绕用户目标，不围绕实现形状：

- 失败提示不泄露内部错误，并按当前语言显示；
- 读取失败不会把已有数据变成“空”；
- 权限失效会隐藏敏感内容；
- 明确未提交允许安全重试，结果未知不会自动重复副作用；
- 服务端 receipt、revision 和 durable 阶段在响应丢失、并发与重启后仍保持一致。

不维护以下测试：源码必须出现某个组件/字段/正则、每个错误文案拆成固定三个属性、截图目录包含每一种错误、浏览器 journal 或 Web Lock 的内部实现。视觉审查只针对真实用户流程中仍有歧义或交互风险的页面。
