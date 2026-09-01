# Nexus 错误提醒与恢复规范

> 状态：当前规范。本文只定义用户可见错误、HTTP 失败事实和安全恢复边界。

## 1. 目标

错误机制只做三件事：

1. 告诉用户哪个目标没有完成；
2. 说明当前最重要的影响；
3. 在确有安全动作时提供一个下一步。

不影响用户任务、数据、权限、可见内容或等待时间的内部故障只写日志，不弹提示。错误判断是确定性产品逻辑，不调用 LLM，也不让模型生成恢复动作。

本规范不追求统一所有领域的事务协议。Agent、Automation、Conversation、Configuration、Goal 和 Workspace 继续拥有各自的 request ID、revision、receipt、round 或 run 身份。

## 2. 用户界面

### 2.1 默认结构

用户可见错误固定收敛为：

- 第一行：具体标题，例如“任务没有保存”，不用“发生错误”或错误码；
- 第二行：一句话合并当前影响和下一步；
- 可选动作：至多一个主动作，例如“重新加载”“核对运行记录”或“重新登录”。

主提示不展示堆栈、SQL、文件路径、Provider 原文、HTTP 状态、内部 ID 或服务端 `detail`。诊断号只在折叠诊断或复制诊断信息中出现。

表单字段错误留在字段附近并保留输入。页面读取失败、区域失败和修改失败分别留在其所属区域，不用全局 toast 代替。需要用户处理、权限变化或结果未知的提示不能自动消失。

### 2.2 常见场景

| 场景 | 标题回答 | 第二行回答 | 动作 |
| --- | --- | --- | --- |
| 输入无效 | 哪个内容未通过 | 如何修改；已有输入保留 | 通常无 |
| 读取失败且无快照 | 哪个内容未加载 | 当前不可用；可重新加载 | 重新加载 |
| 刷新失败但有快照 | 内容可能不是最新 | 现有内容仍显示；可重新加载 | 重新加载 |
| 修改明确未提交 | 哪个修改未生效 | 已有数据未改变；修正后可再试 | 按领域决定 |
| 修改结果未知 | 哪个结果待确认 | 不要重复提交；先读取权威状态 | 核对状态 |
| 登录或权限失效 | 当前内容无权继续访问 | 敏感快照已隐藏；重新验证身份 | 重新登录/验证 |
| 外部投递未知 | 投递结果待确认 | 不自动再发；先检查接收端 | 核对后重投 |

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

字段只有五个：

| 字段 | 含义 |
| --- | --- |
| `version` | 当前固定为 `1` |
| `code` | 稳定的 `domain.reason` 机器分类 |
| `category` | 开放分类，例如 validation、authentication、authorization、conflict、transport、internal |
| `effect` | `not_applicable`、`not_applied`、`accepted`、`committed` 或 `unknown` |
| `transport_request_id` | 可选诊断关联号，仅用于排障 |

`FailureCore` 不包含用户文案、恢复动作、URL、命令或重试等待时间。限流/排队等领域若需要等待信息，使用所属协议；Conversation 历史索引的 `retry_after_ms` 仍是该读取协议的一部分，不属于 FailureCore。

`transport_request_id` 只能复用经过校验的当前 `X-Request-ID`。它不能成为幂等键、授权、路由、缓存键或业务身份，也不能替代任何领域 request ID。

## 4. 后端边界

- 旧 Handler 可继续使用兼容 `WriteFailure`；接入结构化失败的 Handler 显式调用 `WriteError`。
- Handler 只把已经由事务、revision、receipt 或领域状态证明的影响写入 `effect`。无法证明时使用 `unknown`。
- 业务错误用 typed/sentinel error 或明确结果映射；禁止根据 `err.Error()` 文案猜 code、category 或 effect。
- `detail` 必须安全，但对 v1 客户端只是旧协议兼容内容，不参与界面决策。
- 共享 writer 记录 code、阶段、effect、诊断号和 cause 类型；不记录秘密、完整请求正文或外部原始错误。
- `FailureCore` 只在失败响应序列化，不增加成功路径数据库访问，也不建立全局 operation ledger。

## 5. 前端边界

### 5.1 解析与文案

- HTTP 层宽容解析旧 envelope 和 `FailureCore v1`；未知版本或字段不使响应解析失败。
- 结构化失败的用户正文始终使用当前界面的本地化文案，不直接显示服务端 `detail`。
- 共享层只投影 access、code、category 和 effect，不推测领域恢复动作。
- 业务界面根据当前目标选择标题、影响说明和至多一个动作。

### 5.2 读取

- 首次读取失败显示错误，不伪装成空状态。
- 刷新失败保留仍可信的同 scope 快照，并说明内容可能不是最新。
- 401/403 立即隐藏该 scope 的敏感快照和交互层。
- scope、owner、resource identity 和请求代次变化后，旧响应不得写回新页面。

### 5.3 修改

- 当前页面可以防重复点击，并在结果未知时暂时锁住同一动作。
- 浏览器不保存通用 mutation journal，不用 Web Locks 充当服务端并发控制，也不因本地存储不可用拒绝业务操作。
- 页面刷新、多窗口和应用重启后直接读取服务端权威状态；前端影子状态不是真相源。
- 客户端不自动重放创建、修改、删除、运行、投递或工具调用。
- 只有领域 receipt、revision/CAS、durable ACK 或权威读取能证明结果；HTTP 成功/失败、时间经过或诊断号本身不能证明数据影响。

## 6. 重试

- 普通读取可以有限重试；参数、权限、冲突和结果未知不自动重试。
- 明确幂等的领域阶段可以复用同一业务 request ID；每次仍重新鉴权。
- 前端、服务、第三方 SDK 和 worker 之间，一个阶段只能有一个重试负责人。
- 第三方不支持幂等键或结果查询时，响应丢失进入人工核对，不能假装 exactly-once。

## 7. 领域安全边界

### Agent 创建

`creation_request_id`、intent digest 和创建 receipt 仍由 Agent 领域持有。浏览器可保存非敏感 request ID 以便重载后查询，但存储失败不阻止创建。Agent/Profile/Runtime 与 committed receipt 同事务；删除同时写 receipt 墓碑；未知结果只查询 exact receipt，不清理可能已提交的 workspace。

### Automation

`configuration_version`、创建/立即运行 request ID、run、delivery attempt、deletion token 和 heartbeat outbox 仍是服务端真相。运行、终态提交、首次投递、人工重投、删除和 wake 是独立阶段。router 已调用但结果未知时不自动补投；删除无法证明原执行停止时进入 `review_required`。

Web 只保留当前页面的 pending/unconfirmed 状态。重新打开页面直接读取任务、运行历史和权限状态；不恢复浏览器 mutation journal。创建任务的 request ID 可作为 receipt 查询辅助，但不是提交门槛。

### Workspace 与 Memory

文件 revision 保护并发编辑。冲突保留用户草稿；覆盖必须基于最新 revision 并由用户明确选择。上传结果未知且无法由内容摘要或 receipt 证明时停止后续上传，不按文件名猜成功。

### Conversation

Conversation 继续使用独立的 client request/message、Session、round、Agent round、failure code、ACK 和重连对账协议。首次 WebSocket error 不是终态；客户端不自动重发 prompt、工具或副作用命令。提示只出现在 Composer 状态栈，不写入 Feed、历史、未读或滚动几何。

### Goal 与 Configuration

Goal 和 Configuration 继续使用各自的 revision、request ID、plan digest、审计或回执。缺少领域身份时，结果未知只能在当前页锁定和对账，不能用 HTTP 诊断号补造身份。

## 8. 验证原则

验证围绕用户目标，不围绕实现形状：

- 失败提示不泄露内部错误，并按当前语言显示；
- 读取失败不会把已有数据变成“空”；
- 权限失效会隐藏敏感内容；
- 明确未提交允许安全重试，结果未知不会自动重复副作用；
- 服务端 receipt、revision 和 durable 阶段在响应丢失、并发与重启后仍保持一致。

不为以下内容维护测试：源码必须出现某个组件/字段/正则、每个错误文案必须拆成固定三个属性、截图目录必须包含每一种错误、浏览器 journal 或 Web Lock 的内部实现。视觉审查只针对真实用户流程中仍存在歧义或交互风险的页面。
