# Markdown 流式渲染

- `markdown-stream-blocks.ts`: 把不完整输入切成可稳定渲染的区块。
- `markdown-streaming.tsx`: 以一个持久 `MarkdownText` 身份组合静态区块与当前增量区块；本次挂载一旦进入流式态，终态继续按相同 `start_offset` 分块并切换为静态组件，避免代码、图片等后续块重挂载；初次加载的历史消息直接走静态单块。
- `use-smooth-streaming-markdown-content.ts`: 只对追加快照做单调打字机追赶；增量字符原地追加到目标缓冲，终态继续排空已有 backlog，历史回放、非前缀修正或减少动态效果时立即对齐真实正文，同时动态响应低动态设置变化。

流式层只处理时间和增量边界，不复制正文组件语义、工作区路径解析或 Mermaid 渲染状态。
结构化消息入口必须在 live 文本为空时就挂载 MarkdownRenderer；首批正文因此是对空目标的追加，历史或恢复消息则用已有正文首挂并直接呈现。
Room 的阅读位置仍由会话滚动层维护；Markdown 平滑只改善阅读节奏，不得承担滚动补偿或掩盖布局抖动。
