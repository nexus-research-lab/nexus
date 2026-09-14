# Subagent：父智能体内的局部委派

需要子智能体时，复用 `nexus.command`，选择 `domain=subagent`。工具列表没有独立 `Agent` 并不表示无法委派。先读取目标操作的 contract，再按返回的 closed input 调用；不要寻找 CLI、写临时 JSON 或自行拼装 Agent 工具。

## 派生与并行

- 创建前读 `action=contract, operation=spawn`，再用 `action=invoke`、稳定 `request_id` 与 contract 要求的输入提交。任务说明包含范围、已知证据、期望结果和写入边界。
- spawn 固定异步启动通用子智能体。使用实际返回的子任务标识；启动成功只表示已受理，不表示任务完成。
- 独立任务可以依次快速启动后并行运行；父智能体继续处理不依赖子结果的工作。存在依赖时先取得上游结果，再启动下游。
- 子智能体不继承父智能体的完整对话；把任务所需上下文写入任务说明。它不能通过这个入口再次递归派生。

## 读取、补充与收口

`action=inspect`（不传 operation）列出当前父会话子任务。对实际返回的任务标识，按需读 get、wait、send 或 stop contract，再 invoke：get 读取当前状态/输出；wait 最多等待 30 秒；send 追加说明或继续已有任务；stop 停止任务。读取不需要 mutation request_id，send/stop 需要。

优先完成独立工作，再等待仍需的结果；wait 超时只是任务尚未结束，不是失败。send 的 queued 回执只表示消息已接收。完成通知到来后核对实际状态和输出，父智能体负责整合、验证与最终交付；不把子任务完成当成父任务验收。准备结束父任务时，将已启动子任务收口或显式停止。

副作用结果未知时先 inspect/get 对账，不能换新 request_id 自动重放。相同意图在同一物理轮次复用原 request_id；改变任务说明或目标是新意图。回执不跨进程持久化，重启后不能把未查到任务猜成未执行。

## 工作图中的使用

已有 Work Item 的执行者可以在自己的任务内派生子智能体。Room Lead 自己执行时，先取得 self Assignment，再派生。宿主按当前精确 WorkBinding/Assignment 与原始工具调用身份建立 child Attempt；不要手填这些身份，也不要为每个局部子任务另造 Work Item。

没有工作图仍可委派；缺少唯一可绑定责任时，子任务只作为 runtime observation，不能冒充托管 Work Item 证据。多个并行子任务分别跟踪，终态只关闭各自 child Attempt；父智能体仍负责 Submission、Review 与 Acceptance。

## 能力边界

当前入口需要支持 `subagent_control_v1` 的 nxs runtime。若宿主明确返回 capability 不支持，报告当前运行时能力缺失；增加提示词或反复重试不会补齐它，也不要擅自用 CLI 或 Room 成员替代。Plan Mode 不允许启动或续派执行任务；可读取已有任务或停止它们。
