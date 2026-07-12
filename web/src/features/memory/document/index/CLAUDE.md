# Memory Index

本目录只负责解析和渲染 `MEMORY.md` 中指向记忆文档的索引条目。

- `memory-index-model.ts` 解析 Markdown 链接并只接受 `memory/` 作用域路径。
- `memory-index-entries.tsx` 渲染可导航条目，不读取正文或拥有选择状态。
