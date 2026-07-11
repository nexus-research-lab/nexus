# 消息过程领域

- `message-process-summary.ts` 以规则表统计过程指标并提取最近动作摘要。
- `message-question-timeout.ts` 只识别 AskUserQuestion 已超时的工具结果。

本目录只处理过程内容的纯领域投影；展开生命周期留在控制器，过程卡片和样式留在视图。
