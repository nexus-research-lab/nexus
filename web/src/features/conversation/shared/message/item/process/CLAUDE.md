# 消息过程领域

- `message-process-summary.ts` 以规则表投影无语言的过程指标和最近动作结构；当前语言文案只在 Assistant 视图格式化。
- `message-question-timeout.ts` 只识别 AskUserQuestion 已超时的工具结果。
- `dm-tool-run-segments.ts` 供 DM/Room live 共用：同一 process surface 只投影一个以首个 `tool_use.id` 为身份的稳定执行段，中间 text 旁白、普通 thinking 和连续工具均并入其中；最终回复必须先由 final projection 剥离，`preceding_tool_use_ids` 只负责把最新 ephemeral ToolUseSummary 定位到包含该批工具的执行段，不得把连续过程拆成多栏，人工交互和生成式 UI 仍形成边界。

本目录只处理过程内容的纯领域投影；未解析工具与仍在继续的 process 执行段保持 active，final 正文恢复或轮次终态才使该段进入 complete，具体折叠锁存、卡片和样式留在视图。终态 complete 仍保留可展开的审计入口，但不再具有实时进展语义。
