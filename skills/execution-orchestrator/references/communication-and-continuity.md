# 通信与连续执行

只在处理 Agent 间内容传递、状态 command、Bridge 观测或 handoff 连续性时读取本文件。

## 四个平面

| 平面 | 负责什么 | 典型载体 |
| --- | --- | --- |
| 消息 | 发现、上下文、结论、反馈和人类可读结果 | Room `@`、普通消息、父子 Agent 消息、产物 |
| 责任 | 谁对哪个交付负责、交给谁审核 | Work Item、Assignment、ReviewBinding |
| 状态 | 提交、验收、阻塞、接管、重规划等权威事实 | round-scoped Nexus Goal/Execution command |
| 观测 | 实际调用了哪个 Agent、Subagent、Tool、Gate 或 Hook | Bridge/runtime lifecycle events |

不要让一个平面冒充另一个平面。`@` 可以携带完整结果，但不会凭文字创建 trusted Assignment；command 可以记录交付状态，但不应吞掉真正的研究内容；Bridge 自动记录运行活动，模型不需要为了“让图有节点”执行状态操作。

## Room handoff

需要持久责任时，先用结构化 Assignment 确立 owner 和 reviewer，再通过 Room 消息或定向消息传递实质上下文。完成后，内容通过消息或产物交回，Submission/Review operation 只记录不可变的交付与验收事实。

Lead 应从任务实际结构判断是否需要持久成员责任，而不是检查用户有没有说“协作”。若完成任务需要多个持久成员分别拥有可追责交付，或存在依赖、并行、汇总、验收、恢复与连续性交接，就先准备完整 Plan Document，并提交服务端返回的 exact sealed proposal，再根据刷新后的 `ready_work` 创建 Assignment。图 materialize 前 `assign_work` 不可用是正常 bootstrap，不是改用裸 `@` 派活的理由。

只想聊天、brainstorm、投票或获取一次性帮助时，直接使用普通消息和 `@`，不要创建 WorkGraph。成员名后的空格只影响可读性，后端会按已知别名匹配；不要依赖 mention 文本伪造 binding。

父 Agent 与 Subagent 之间遵循同样原则：父 Agent 下发边界和已有证据，子 Agent 返回局部结果，父 Agent 负责整合与最终提交。

子智能体不要求先创建 WorkGraph。存在唯一可信 Assignment 与 runtime correlation 时，后端会自动把它记为 managed child Attempt；普通对话、局部探索或无法唯一挂接时仍可由 Agent 自主调用，只作为 runtime-only 子图出现，不冒充正式交付证据。是否调用、并行几个以及何时整合由 Agent 根据上下文隔离价值决定，Hook 不用 Plan 形状替 Agent 做选择。

## 连续执行

- 节点启动后持续消费 command/工具结果、子智能体结果、已验收依赖和 review 反馈，直到真实交付、具体外部阻塞或终态。
- Handoff、Submission 或 Acceptance 是状态转移，不是自动结束当前回答的理由。当前 round 仍有授权且出现 Ready 工作时，继续推进。
- 只有真正需要用户选择、权限、材料或外部状态变化时才停下来请求输入；不要要求用户发送“继续”。
- 消息首先展示研究、分析、代码、证据、决策和结果。Graph UI 已经展示运行状态，不要用大段“已分派、正在等待、下一步”替代任务内容。
