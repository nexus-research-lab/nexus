# 消息过程领域

- `message-process-summary.ts` 以规则表投影无语言的过程指标和最近动作结构；当前语言文案只在 Assistant 视图格式化。
- `message-question-timeout.ts` 只识别 AskUserQuestion 已超时的工具结果。
- `dm-tool-run-segments.ts` 只为 DM live 把普通连续工具调用投影成以首个 `tool_use.id` 为身份的稳定过程段；过程段保留计数与阶段事实，不持有本地化摘要，人工交互和叙事内容必须形成边界，Room 不消费该模型。

本目录只处理过程内容的纯领域投影；未解析工具保持 active，已解析工具段在后续叙事、final 正文恢复或轮次终态进入 complete，具体折叠锁存、卡片和样式留在视图。
