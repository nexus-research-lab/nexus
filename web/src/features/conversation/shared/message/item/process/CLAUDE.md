# 消息过程领域

- `message-process-summary.ts` 以规则表投影无语言的过程指标和最近动作结构；当前语言文案只在 Assistant 视图格式化。
- `message-question-timeout.ts` 只识别 AskUserQuestion 已超时的工具结果。
- `dm-tool-run-segments.ts` 供 DM/Room live 共用：每段无可见 text 打断的普通 thinking 与连续工具投影为一个以首个 `tool_use.id` 为身份的稳定执行段；text 必须原位可见并结束前一工具段，最终回复先由 final projection 剥离。`preceding_tool_use_ids` 只负责把最新 ephemeral ToolUseSummary 定位到包含该批工具的执行段，尚未完成的人工交互和生成式 UI 仍形成边界，已有结果的人机提问回归普通工具段。

本目录只处理过程内容的纯领域投影；未解析工具与仍在继续的 process 执行段保持 active，final 正文恢复或轮次终态才结束该段，终态只由最后一个工具动作决定。早期失败、拒绝或替换若已被后续成功动作恢复，只保留在计数与展开审计里，不能污染整段终态；具体折叠锁存、卡片和样式留在视图。终态 complete 仍保留可展开的审计入口，但不再具有实时进展语义。
