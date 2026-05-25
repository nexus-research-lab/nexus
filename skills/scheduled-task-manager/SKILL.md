---
name: scheduled-task-manager
description: 创建、查看、检查、修复、启停、运行、更新或删除 Nexus 定时任务时使用。尤其当用户用自然语言说“提醒我”“每天/每周/每隔一段时间”“定时做某事”“检查发送情况”“停止任务”时，先用本 skill 决策，再调用 nexus_automation MCP 工具。
---

# scheduled-task-manager

你负责把用户的自然语言需求稳定转换成 `nexus_automation` 定时任务工具调用。不要把底层字段丢给用户，也不要在工具报错后反复猜参数。

## 必须遵守

1. 用户要创建定时任务时，先按本文判断，再调用 `create_scheduled_task`。
2. 用户要检查、修改、停止、恢复或补发任务时，先定位真实任务和 run，不要凭名字猜；管理工具支持直接传 `query`，但只有唯一命中才会执行，多候选时必须让用户确认。
3. 短提醒任务直接创建，不要询问 `execution_mode` / `reply_mode`。
4. 复杂任务缺少上下文时，先问用户，不要猜。
5. 工具报错后最多修正一次明确错误；如果仍失败，直接把错误说明给用户。
6. 向用户解释时使用产品语义，例如“执行会话”“结果回传”“投递目标”“发送失败”，不要讲 `session_target`、`delivery` 这类内部结构。

## 快速判断

**短提醒任务**满足全部条件时直接创建：

- 指令不超过 24 个中文字符或 24 个英文单词左右
- 只是提醒、打卡、喝水、站立、吃药、会议等轻量动作
- 不需要读取历史上下文、总结、分析、汇报、监控、整理、复盘

短提醒默认：

- `execution_mode` 不填
- `reply_mode` 不填
- 让工具默认当前会话可见：复用当前会话执行，并把结果回到执行会话

**复杂任务**命中任一条件且缺少明确执行/回传意图时必须确认：

- 包含“总结、汇总、报告、分析、研究、整理、复盘、检查、监控、跟进”
- 需要读取当前会话、Room、公区、文件、历史消息、外部系统
- 用户明确希望结果发回某个会话、群、Room、频道
- 任务会长期维护状态或上下文

例外：当前 DM/Room 中，用户说“每天搜索新闻发给我/告诉我/通知我”这类不依赖当前聊天历史的独立任务时，可直接创建；工具会默认临时执行并把结果回投当前会话。若用户说“总结这个对话/读取聊天记录/基于当前上下文”，仍必须显式确认执行会话，不能默认隔离执行。

复杂任务至少确认两件事：

- 执行在哪里进行：主会话、当前会话、临时会话、专用长期会话
- 结果发到哪里：不回传、回到执行会话、回到指定会话、投递给智能体、投递到 IM/飞书群

### 后台工具能力预检

定时任务到点后是无人值守后台运行，不能等待前端权限弹窗，也不能用 `AskUserQuestion` 继续补问。

- 新闻、搜索、资料收集类任务通常需要目标 Agent 允许 `WebSearch` / `WebFetch`。
- 读写 workspace 文件的任务通常需要目标 Agent 允许 `Read` / `Write` / `Edit` / `Bash` 中的对应工具。
- 如果用户要求的任务明显需要某个工具，而你不确定目标 Agent 是否已允许该工具，创建后要明确提醒：若后台工具未预授权，该 run 会失败并能在 `get_scheduled_task_status` / `get_scheduled_task_daily_report` 里看到原因。
- 在飞书/IM 群里缺少必要上下文时，用普通文本回复让用户补充，不要调用 `AskUserQuestion`。

## 执行会话与回传决策

先判断“任务是否需要当前上下文”，再判断“用户是否需要看见结果”。

### 默认选择

| 场景 | 执行会话 | 结果回传 | 工具参数 |
|---|---|---|---|
| 简单提醒、打卡、吃药、喝水、开会提醒 | 当前会话 | 回到当前会话 | 不填 `execution_mode` / `reply_mode`，由工具默认 |
| 要继续当前对话/Room 的上下文做事 | 当前会话 | 回到当前会话 | `execution_mode="existing"` + `reply_mode="execution"` |
| 独立任务，不需要当前上下文，但用户要看到结果 | 临时会话 | 回到当前/指定会话 | `execution_mode="temporary"` + `reply_mode="selected"` + `selected_reply_session_key` |
| 独立任务，需要投递给某个智能体 | 临时会话 | 智能体收件箱 | `execution_mode="temporary"` + `reply_mode="agent"` + 可选 `reply_agent_id` |
| 独立任务，需要投递到飞书/IM 群 | 临时会话 | 指定通道 | `execution_mode="temporary"` + `reply_mode="channel"` + `reply_channel` + `reply_to` |
| 高频、噪音、后台维护，用户明确不需要看结果 | 临时会话 | 不回传 | `execution_mode="temporary"` + `reply_mode="none"` |
| 长期复用同一个任务上下文 | 专用长期会话 | 视需求回传 | `execution_mode="dedicated"` + `named_session_key`，需要可见结果时用 `reply_mode="selected"` |
| 主智能体主线系统事件 | 主会话 | 不额外回传 | `execution_mode="main"` + `reply_mode="none"`，仅主智能体可用 |

### 什么时候必须回传

- 用户说“提醒我、告诉我、发给我、回到这里、发到这个 Room/会话/群”。
- 定时任务的价值就是通知用户，例如喝水、吃药、会议、到点回电话。
- 任务产物需要用户阅读，例如日报、周报、总结、检查结果、告警。
- 用户从当前 DM 或 Room 里创建任务，且没有明确说“后台执行不用告诉我”。
- 用户在当前 DM/Room 里要求“每天搜索新闻发给我/告诉我/通知我”，且任务不依赖当前聊天历史；工具会默认 `execution_mode="temporary"` + `reply_mode="selected"` 回投当前会话。
- 用户从飞书/IM 群里创建任务，且要求“发到群里/发到这里/每天推送”，优先把结果投递回该 IM 群。
- 当前会话是结构化飞书/IM 群且用户说“每天推送/发送/播报新闻”这类明确可见投递意图时，工具会兜底推导为临时执行并投递回当前群；如果用户说“不要推送/静默运行”，必须改成显式回传策略或先确认。你仍应优先显式传 `execution_mode="temporary"` + `reply_mode="channel"`。

### 什么时候可以不回传

- 用户明确说“后台执行、不用通知、不用发回来、只记录、静默运行”。
- 任务只是维护状态、预热、清理、同步、刷新缓存、写入内部文件。
- 任务结果会由另一个明确的外部渠道发送，当前会话不需要重复显示。
- 任务非常高频且每次输出都没有用户价值。

### 什么时候用智能体收件箱

- 用户只说“投递给某个智能体”或“让某个智能体每天收到”，没有指定具体会话/群。
- 当前环境没有可用的 IM 群 chat_id，但任务产物需要长期可见。
- 不考虑飞书群时，日报/周报/监控摘要至少要能投递到某个智能体。

使用：

```json
{
  "execution_mode": "temporary",
  "reply_mode": "agent",
  "reply_agent_id": "<接收智能体 id>"
}
```

`reply_agent_id` 不填时默认投递给任务目标智能体。

### 什么时候用外部通道/飞书群

- 用户明确说“发到飞书群/这个群/IM 群/频道”。
- 当前会话来自飞书群，且能拿到结构化 session key 或 chat_id。

优先使用结构化会话 key：

```json
{
  "execution_mode": "temporary",
  "reply_mode": "channel",
  "reply_session_key": "agent:<agent_id>:fs:group:<chat_id>"
}
```

没有结构化 key 但知道飞书 `chat_id` 时：

```json
{
  "execution_mode": "temporary",
  "reply_mode": "channel",
  "reply_channel": "feishu",
  "reply_to": "<chat_id>",
  "reply_account_id": "chat_id"
}
```

如果当前会话就是目标飞书/IM 群，而用户说“发到这个群/每天推送/每天发送/每天播报”，可以传 `reply_mode="channel"`，或只传与当前会话一致的 `reply_channel`；工具会用当前结构化会话补齐真实群目标。不要在当前是飞书群时凭群名另猜 `reply_to`。

如果用户要求投递到飞书但你不知道群 `chat_id` 或当前群身份，先问用户或要求他在目标飞书群里发起这条指令；不要凭群名猜。

### 飞书接口不确定时

Nexus 运行时投递必须走 `nexus_automation` 和内置 Feishu channel，不要直接用外部 MCP 绕过定时任务 run ledger、投递重试和日报统计。

如果只是需要理解飞书接口、权限或错误码，可使用官方 `larksuite/lark-openapi-mcp`：

- 文档召回 MCP：`@larksuiteoapi/lark-mcp recall-developer-documents`
- OpenAPI MCP：`@larksuiteoapi/lark-mcp mcp`
- 项目说明：`docs/specs/feishu-openapi-mcp-integration.md`

使用边界：

- `feishu tenant_access_token failed`：检查 App ID / App Secret、应用状态和 tenant token 文档。
- `feishu send message failed`：根据错误码检查 `im.v1.message.create` 权限、`receive_id_type`、目标 `chat_id/open_id`。
- `delivery_dead_letter_at` 已出现：先修配置或目标，再调用 `retry_scheduled_task_delivery` 补发。
- 不要用 `lark-mcp` 直接发送定时任务结果；那样不会写入 Nexus 的 delivery 状态和审计。

### 什么时候用临时会话

临时会话等价于 OpenClaw 的 isolated cron：干净执行、不污染当前历史。适合：

- 不需要读取当前会话历史的独立任务。
- 高频或噪音任务，避免把当前会话刷满。
- 重分析、长报告、定期检查这类应当独立完成的任务。
- 每次执行都应该从干净上下文开始，而不是继承上一次对话。

如果临时会话的结果要让用户看见，不要用 `reply_mode="execution"`；使用 `reply_mode="selected"` 并把 `selected_reply_session_key` 设为当前或用户指定的会话。

### 什么时候用当前会话

- 用户说“这个对话/这个 Room/当前上下文/接着刚才”。
- 任务需要读取当前会话或 Room 公区历史。
- 任务是一个可见提醒，且用户没有要求隔离执行。
- 用户希望执行过程和结果都留在同一个会话里。

当前会话模式可以不传 `selected_session_key`，工具会使用当前会话。
在 Room 中调用时，当前会话指的是 Room 共享会话，不是某个成员 Agent 的私有运行会话；用户说“发回这个 Room/这个群/这里”时优先复用这个默认值。

### 什么时候用专用长期会话

- 任务有自己的长期状态，例如“每天跟进 A 项目进展”。
- 多次执行之间需要保留任务内部上下文，但不应污染用户当前聊天。
- 用户明确希望“给这个任务开一个固定会话/长期会话”。

专用长期会话如果要让用户看到结果，优先 `reply_mode="selected"`，不要默认静默。

## 一句话解析规则

从用户句子提取三件事：

- `name`: 用户明确给出的任务名；没有则用动作生成短名称
- `instruction`: 到点要做什么，只写执行动作，不要夹带调度描述
- `schedule`: 频率或时间

示例：

用户：“test 每分钟提醒我喝水”

```json
{
  "name": "test",
  "instruction": "提醒我喝水",
  "schedule": {
    "kind": "interval",
    "interval_value": 1,
    "interval_unit": "minutes"
  }
}
```

不要为这个例子补 `execution_mode` 或 `reply_mode`。

## 创建模板

### 每隔一段时间

用户：“每 30 分钟提醒我站起来”

```json
{
  "name": "站立提醒",
  "instruction": "提醒我站起来",
  "schedule": {
    "kind": "interval",
    "interval_value": 30,
    "interval_unit": "minutes"
  }
}
```

`interval_unit` 只用 `seconds` / `minutes` / `hours`。

### 每天固定时间

用户：“每天早上 9 点提醒我开晨会”

```json
{
  "name": "晨会提醒",
  "instruction": "提醒我开晨会",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

工作日用：

```json
{
  "kind": "daily",
  "daily_time": "09:00",
  "weekdays": ["mon", "tue", "wed", "thu", "fri"]
}
```

### 单次任务

用户：“明天下午 3 点提醒我给客户回电话”

```json
{
  "name": "客户回电提醒",
  "instruction": "提醒我给客户回电话",
  "schedule": {
    "kind": "single",
    "run_at": "2026-04-29T15:00"
  }
}
```

用户说“今天、明天、下周”时，按当前本地时区理解。没有其他说明时使用上海时间。

### Cron

只有当用户明确给 cron 表达式，或 daily/interval 表达不了时才用：

```json
{
  "kind": "cron",
  "expr": "0 9 * * 1-5"
}
```

cron 必须是 5 段，且需要能被 UI 编辑：minute/hour 是单个整数，day-of-month 和 month 是 `*`。

## 复杂任务模板

用户：“每天 9 点总结这个 Room 昨天的进展并发回来”

先确认：

```text
你希望这个定时任务在哪个会话里执行？结果要回到哪里？
```

如果用户选择当前会话执行并回到当前会话：

```json
{
  "name": "每日进展总结",
  "instruction": "总结这个 Room 昨天的进展",
  "execution_mode": "existing",
  "reply_mode": "execution",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

当前会话存在时，`selected_session_key` 可不填，工具会使用当前会话。
在 Room 场景里这里的当前会话会解析为共享 Room，因此结果会留在公区对话里。

如果用户选择隔离执行但结果仍发回当前会话：

```json
{
  "name": "每日进展总结",
  "instruction": "总结这个 Room 昨天的进展",
  "execution_mode": "temporary",
  "reply_mode": "selected",
  "selected_reply_session_key": "<当前会话 key>",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

如果用户明确要求后台静默执行：

```json
{
  "name": "后台状态同步",
  "instruction": "同步项目状态到内部记录",
  "execution_mode": "temporary",
  "reply_mode": "none",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

### 每日新闻并投递到飞书群

用户：“每天 9 点搜索新闻并发到这个飞书群”

预检：目标 Agent 需要能使用 `WebSearch` / `WebFetch`。如果不确定，创建成功后也要提示用户检查 Agent 工具权限；后台缺权限时可用日报或状态工具定位失败 run。

如果当前就是飞书群上下文，可直接让工具回投当前群：

```json
{
  "name": "每日新闻摘要",
  "instruction": "搜索今天的重要新闻，整理为简洁摘要并给出来源链接",
  "execution_mode": "temporary",
  "reply_mode": "channel",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

如果已经知道结构化 session key，也可以显式传 `reply_session_key="agent:<agent_id>:fs:group:<chat_id>"`。

如果没有飞书群目标，但用户接受投递给智能体：

```json
{
  "name": "每日新闻摘要",
  "instruction": "搜索今天的重要新闻，整理为简洁摘要并给出来源链接",
  "execution_mode": "temporary",
  "reply_mode": "agent",
  "reply_agent_id": "<接收智能体 id>",
  "schedule": {
    "kind": "daily",
    "daily_time": "09:00"
  }
}
```

不要用 `reply_mode="none"` 创建用户期待阅读的日报、新闻、检查报告或告警任务。

## 检查、修复、修改和停止

### 检查今天发送情况

用户问“今天的定时任务发送情况/日报有没有发/有没有失败”：

1. 先调用 `get_scheduled_task_daily_report`，缺省查今天；主智能体可带 `agent_id`，知道具体任务时带 `job_id`。如果当前就在飞书/IM 群里且用户泛化询问“今天发送情况/有没有失败”，不要先列全局任务；日报工具会默认聚合当前群相关任务。如果用户在 DM/Room/IM 群里明确问“这里/当前会话/这个群的发送情况”，可把原话放进 `query`；日报工具会聚合当前会话相关任务。
2. 先读日报任务上的 `signals`、`latest_execution_error`、`latest_delivery_error`、`execution_failed_run_ids`、`delivery_pending_run_ids`、`delivery_skipped_run_ids`、`manual_redelivery_run_ids`、`delivery_dead_letter_run_ids` 和 `recovery_run_id`；这些字段已经是可操作的 run id 和可直接解释的错误摘要。
3. 如果报告里有 `delivery_failed_run_count`、`delivery_dead_letter_run_count`、`running_task_count` 或任务 `signals`，再对具体任务调用 `get_scheduled_task_status` 查看更完整的错误与事件。
4. 回答时说清楚：任务数、运行次数、执行失败数、投递失败数、待投递/跳过投递/死信 run、最近错误。
5. 需要追溯“谁改过/停过/补投过”时，再调用 `get_scheduled_task_events`。
6. 如果用户说的是已删除任务或 `list_scheduled_tasks` 找不到候选，先用 `search_scheduled_task_history.query` 按名称/关键词找回 `job_id`，再带 `job_id` 查日报、run 或审计。
7. 在 DM/Room/飞书/IM 群里，用户说任务名但没说“当前会话/这个群”时，仍优先按当前会话相关任务理解；只有当前会话没有匹配时才会回退到当前权限范围内的其他任务。用户明确说“这里/当前会话/这个群/当前频道/这个任务/当前任务”的当前或已删除任务时，可直接把这类词放进 `list_scheduled_tasks.query`、`search_scheduled_task_history.query`、`get_scheduled_task_status.query`、`get_scheduled_task_daily_report.query`、`get_scheduled_task_runs.query`、`get_scheduled_task_events.query`、`disable_scheduled_task.query` 等工具；工具会强制限定到当前会话，再用剩余词匹配任务名和内容。日报里如果只是“这里/当前会话/这个群的定时任务发送情况”这种泛化问法，会聚合当前会话任务；如果带具体任务名时仍定位唯一任务。
8. 如果当前会话日报返回 `tasks=[]` 且 `totals.task_count=0`，这是正常空结果，不要继续查全局任务；直接告诉用户当前会话没有相关定时任务或今天没有发送记录。

### 投递失败或漏发

当 `get_scheduled_task_status` 或日报显示投递失败：

1. 优先读取日报任务的 `manual_redelivery_run_ids` / `delivery_dead_letter_run_ids`，或状态健康摘要里的 `health.manual_redelivery_run_ids` / `health.delivery_dead_letter_run_ids`；没有这些字段时再从失败 run 里找 `run_id`。
2. 先判断 `latest_delivery_error` / `delivery_error` 是临时错误，还是投递目标或通道配置错误。目标或配置明显不对时，先修复：改投递目标用 `update_scheduled_task`；飞书应用、权限、凭据问题先让用户修通道配置。
3. 如果用户要求“补发/再发一次/重试发送”，在确认目标/配置已经修好后调用 `retry_scheduled_task_delivery`。只有一个可补投递失败 run 时可不传 `run_id` 让工具自动选择；有多个候选时必须传用户确认的 `run_id`。不要在同一个坏目标上盲目重复重试。
4. 重投递成功后，再调用 `get_scheduled_task_status` 或 `get_scheduled_task_daily_report` 确认 `delivery_status=succeeded`，并向用户说明补发到了哪个投递目标。
5. 如果只是检查，不要自动重投递；先把失败原因、可重投递的 run 和建议修复动作告诉用户。

典型修复顺序：

1. `get_scheduled_task_daily_report` 找到 `manual_redelivery_run_ids` 和 `latest_delivery_error`。
2. `get_scheduled_task_status` 查看失败 run、当前 `delivery_to` 和建议工具。
3. 如果目标错了，`update_scheduled_task` 修正 `reply_mode` / `reply_agent_id` / `reply_channel` / `reply_to`。
4. `retry_scheduled_task_delivery` 补发失败投递，不重新执行任务；只有一个候选可省略 `run_id`，多个失败 run 时带原失败 `run_id`。
5. `get_scheduled_task_status` 验证失败提示消失。

### 执行失败或后台工具未授权

当日报或状态里出现 `latest_execution_error`、`execution_failed_run_ids` 或 `recent_execution_failed`：

1. 先区分执行失败和投递失败。`latest_execution_error` 属于任务本身没跑完；不要调用 `retry_scheduled_task_delivery`，那只补发已经成功执行但投递失败的 run。
2. 如果错误里有“未授权工具”“权限策略拒绝”或工具名（例如 `WebSearch` / `WebFetch` / `Write` / `Bash`），说明后台运行需要的工具没有预授权。告诉用户要么在目标 Agent 的允许工具里加入该工具，要么把任务改成不需要该工具。
3. 用户修好工具权限后，如要补今天的结果，调用 `run_scheduled_task` 重新执行一次；不要只重投递旧失败 run。
4. 用户要改任务本身时，用 `update_scheduled_task` 修改 `instruction`、执行会话或投递目标；改完再按需 `run_scheduled_task` 验证。
5. 如果错误是 `AskUserQuestion`，说明后台任务缺少必要上下文。让用户把缺失信息写进任务配置，或改为会投递到当前会话的可见任务；不要让无人值守任务继续等待交互。

### 运行卡住

当任务 `running=true` 或健康信号建议 `recover_scheduled_task`：

1. 先用日报任务的 `recovery_run_id`，或用 `get_scheduled_task_status` 确认 `health.recovery_run_id` / 任务上的 `running_run_id`。
2. 用户要求恢复、释放、停止卡住任务时，调用 `recover_scheduled_task`，带上 `run_id` 避免误释放。
3. 恢复会先中断真实执行会话，再把未完成 run 标记为 `cancelled` 并重新安排后续调度。

### 修改任务

用户说“改时间/改内容/改投递目标/不要再发送结果”：

1. 如果用户描述足够明确，可直接把描述放进 `update_scheduled_task.query`；工具只有唯一命中当前未删除任务才会修改。
2. 如果用户在 DM/Room/飞书/IM 群里说“新闻日报/监控任务”，直接把原话放进 `query`；工具会优先当前会话相关任务。如果用户明确说“当前会话的新闻日报/这里的监控任务/这个群的新闻日报/这个任务”，这类词会强制限定为当前会话。
3. 如果名称不唯一或工具返回多候选，调用 `list_scheduled_tasks.query` 展示候选并让用户确认。
4. 调用 `update_scheduled_task` 只传要改的字段；不要只传 `job_id` / `query`，用户没有说清楚改什么时先追问。
5. 用户说“再加一条要求/以后也要/补充任务细节”时用 `instruction_append`，不要把这段短要求当成完整 `instruction` 覆盖原任务；只有用户明确要求重写任务内容时才传完整 `instruction`。
6. 单独修改投递时不需要重写 `execution_mode`：例如关闭投递只传 `reply_mode="none"`；改发当前 DM/Room 可传 `reply_mode="selected"`，工具会补齐当前会话；改发当前飞书/IM 群可传 `reply_mode="channel"` 或与当前通道一致的 `reply_channel`，工具会补齐当前群目标；改发其他群时必须传 `reply_to` 或 `reply_session_key`。工具也会根据 `reply_channel` / `reply_to` / `reply_session_key` 推断为通道投递，根据 `reply_agent_id` 推断为智能体收件箱投递。

用户在飞书/IM 群里泛化问“有哪些定时任务/列一下任务/暂停的任务有哪些”时，优先直接调用 `list_scheduled_tasks`，可带 `enabled`；未传 `agent_id/query` 时工具会默认只列当前群相关任务，不会把其他群任务混进来。

### 停止任务

- 用户说“暂停、不要再触发、先关掉”：可直接调用 `disable_scheduled_task`，传 `job_id` 或明确的 `query`；在 DM/Room/飞书/IM 群里会优先当前会话任务，“这里/当前会话/这个群/当前频道”会强制限定到当前会话；工具唯一命中才会停用，保留配置且不打断当前 active run。
- 已暂停任务不会继续自动触发，也不会自动补投递失败 run；如果用户只是要“现在补跑一次/验证一次”，可调用 `run_scheduled_task`，它不会重新启用后续排程。若只是补发已经成功执行但投递失败的结果，先确认投递目标/通道配置，再显式调用 `retry_scheduled_task_delivery`。
- 用户说“恢复、重新启用、继续每天发、打开这个任务”：这是恢复已暂停的任务，调用 `enable_scheduled_task`，传 `job_id` 或明确的 `query`；不要调用 `recover_scheduled_task`，后者只用于释放卡住的 running run。
- 用户说“停止当前正在跑的这次、马上停、别继续跑”：调用 `disable_scheduled_task` 时传 `cancel_active_run=true`；如果已拿到 `running_run_id`，一并传 `run_id`。
- 用户说“删除、移除、彻底不要了”：可直接调用 `delete_scheduled_task`，传 `job_id` 或明确的 `query`；如果返回多候选，先让用户确认；如果返回 `cancelled_active_run=true`，告诉用户删除时已取消正在运行的 run。
- 删除任务会把该任务仍待补发的失败投递 run 立即转为死信；删除后的日报可解释失败，但不要再建议 `retry_scheduled_task_delivery`，需要继续发送时应重新创建任务或改用仍存在的任务。
- 停用或删除后，如用户问历史且没有 job_id，先用 `search_scheduled_task_history` 按任务名找候选；再用 `get_scheduled_task_runs` 查看运行/投递记录，使用带 `job_id` 的 `get_scheduled_task_daily_report` 查某天发送情况，使用 `get_scheduled_task_events` 查看审计记录；已删除任务的日报明细会带 `deleted=true`。

## 出错处理

遇到这些错误时按固定方式处理：

- 缺 `execution_mode` / `reply_mode` 且任务明显复杂: 问用户执行会话和结果回传，不要继续猜。
- 缺 `execution_mode` / `reply_mode` 且只是短提醒: 当前可能不是活跃聊天上下文，告诉用户需要指定执行会话和结果回传位置。
- `interval_value must be a positive integer`: 只检查一次是否把数字放进 `schedule.interval_value`，并确保是整数；修正一次后仍失败就报告错误。
- 需要 `selected_session_key`: 如果用户说“当前会话”，直接用当前会话；否则让用户选择已有会话。
- cron 无法翻译到 UI: 改用 `daily` 或 `interval`；如果表达不了，告诉用户当前 UI 不支持这种 cron。
- 后台工具未授权: 先解释这是执行失败，不是发送失败；让用户授权目标 Agent 的工具，或用 `update_scheduled_task` 改任务；需要补今天结果时用 `run_scheduled_task` 重新跑。
- 飞书投递报错: 先用 `get_scheduled_task_status` 找到失败 run 和 `delivery_error`；如果是接口/权限不确定，按“飞书接口不确定时”的方式查询官方文档；修好后再 `retry_scheduled_task_delivery`。

## 常用工具顺序

1. 创建前通常不需要列任务，除非用户说“替换、修改、删除、同名任务、已有任务”。
2. 创建：`create_scheduled_task`
3. 查看：`list_scheduled_tasks`
4. 查当前或已删除任务候选：`search_scheduled_task_history`
5. 查今天发送情况：`get_scheduled_task_daily_report`
6. 查单个任务状态：`get_scheduled_task_status`
7. 查运行历史：`get_scheduled_task_runs`
8. 查管理审计：`get_scheduled_task_events`（包含自动投递重试 `auto_retry_delivery`）
9. 修改：`update_scheduled_task`
10. 停用/启用：`disable_scheduled_task` / `enable_scheduled_task`
11. 立即跑一次：`run_scheduled_task`
12. 恢复卡住 run：`recover_scheduled_task`
13. 补发失败投递：`retry_scheduled_task_delivery`
14. 删除：先 `list_scheduled_tasks` 确认，再 `delete_scheduled_task`

## 参数对照

只在工具调用时使用这些字段：

- 任务名称: `name`
- 查找或直接管理候选任务: `list_scheduled_tasks.query` 可按 `job_id` / `name` / `instruction` / 投递目标 / 状态模糊过滤，`list_scheduled_tasks.enabled` 可只看启用或停用任务；`update_scheduled_task` / `disable_scheduled_task` / `enable_scheduled_task` / `delete_scheduled_task` / `run_scheduled_task` / `get_scheduled_task_status` 也可直接传 `query`，但只会在唯一命中当前未删除任务时执行
- 查找历史候选: `search_scheduled_task_history.query` 可按当前任务和已删除任务的 `job_id` / 名称 / 任务内容 / 投递目标 / 来源 / 审计 detail 搜索；在 DM/Room/飞书/IM 群里会优先当前会话任务，也可带“这里/当前会话/这个群/当前频道”强制限定到当前会话；`get_scheduled_task_daily_report` / `get_scheduled_task_runs` / `get_scheduled_task_events` 也可传 `query` 定位唯一当前或已删除任务
- 任务指令: `instruction`
- 追加任务指令: `instruction_append`
- 目标智能体: `agent_id`，缺省当前智能体
- 单次: `schedule.kind="single"` + `run_at`
- 每天: `schedule.kind="daily"` + `daily_time` + 可选 `weekdays`
- 间隔: `schedule.kind="interval"` + `interval_value` + `interval_unit`
- cron: `schedule.kind="cron"` + `expr`
- 执行会话: `execution_mode` = `main` / `existing` / `temporary` / `dedicated`
- 执行类型: `execution_kind` = `agent` / `script`，缺省 `agent`
- 结果回传: `reply_mode` = `none` / `execution` / `selected` / `agent` / `channel`
- 指定回传会话: `selected_reply_session_key`
- 投递到智能体: `reply_agent_id`
- 投递到 IM/外部通道: `reply_session_key`，或 `reply_channel` + `reply_to` + 可选 `reply_account_id` / `reply_thread_id`
- 重叠策略: `overlap_policy` = `skip` / `allow`，缺省 `skip`
- 创建后启用: `enabled`，缺省 `true`
