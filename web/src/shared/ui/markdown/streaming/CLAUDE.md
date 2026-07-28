# Markdown 流式渲染

- `markdown-stream-blocks.ts`: 把不完整输入切成可稳定渲染的区块。
- `markdown-streaming.tsx`: 组合静态区块与当前增量区块。
- `use-smooth-streaming-markdown-content.ts`: 直接呈现 runtime 已接收的快照，不在前端叠加打字机节奏。

流式层只处理时间和增量边界，不复制正文组件语义、工作区路径解析或 Mermaid 渲染状态。
Room 的阅读位置由会话滚动层维护；Markdown 层不得用逐字符播放掩盖布局抖动。
