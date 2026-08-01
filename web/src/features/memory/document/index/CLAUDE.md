# Memory Index

本目录只负责解析和渲染 `MEMORY.md` 中指向记忆文档的索引条目。

- `memory-index-model.ts` 解析 Markdown 链接并只接受 `memory/` 作用域路径。
- `memory-index-entries.tsx` 只渲染标题与摘要；路径只用于导航，不在每条索引中重复展示，也不使用表格式分隔线。
