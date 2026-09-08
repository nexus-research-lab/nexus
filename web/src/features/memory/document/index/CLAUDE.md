# Memory Index

本目录只负责解析和渲染 `MEMORY.md` 中指向记忆文档的索引条目。

- `memory-index-model.ts` 解析 Markdown 链接并只接受 `memory/` 作用域路径。
- `memory-index-entries.tsx` 只用共享 dense ListRow 渲染标题与单行摘要；路径只用于导航，不重复展示链接图标、路径或表格式分隔线。
