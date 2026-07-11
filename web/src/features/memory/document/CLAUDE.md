# Memory Document

本目录负责单个 `agentId:path` 记忆文档的读取、实时更新、编辑和保存。

## 边界

- `use-memory-document-state.ts` 只拥有作用域状态与受保护的状态提交入口。
- `use-memory-document-resource.ts` 只处理 HTTP 读取和 SDK 实时内容，旧请求不得覆盖新作用域。
- `use-memory-document-save.ts` 只处理保存互斥与保存期间继续编辑的合并规则。
- `use-memory-document.ts` 组合资源和命令，向视图返回具体控制面。
- Panel、Header 与索引条目分别渲染正文、操作栏和索引导航。

SDK 实时内容优先于旧 HTTP 响应。保存完成只更新已提交草稿对应的基线；用户在保存期间产生的新草稿必须保留。
