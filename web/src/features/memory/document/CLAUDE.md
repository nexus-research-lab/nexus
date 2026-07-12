# Memory Document

本目录负责单个 `agentId:path` 记忆文档的读取、实时更新、编辑和保存。

## 边界

- `use-memory-document-state.ts` 只拥有作用域状态、受保护的状态提交入口，以及保存响应与当前草稿的纯合并规则。
- `use-memory-document-resource.ts` 只处理 HTTP 读取和 SDK 实时内容，旧请求不得覆盖新作用域。
- `use-memory-document-save.ts` 用不可变令牌固定保存请求的 Agent、路径和草稿，并只编排保存事务。
- `use-memory-document.ts` 组合资源和命令，向视图返回具体控制面。
- Panel、Header 与索引条目分别渲染独占正文状态、操作栏和索引导航。

SDK 实时内容优先于旧 HTTP 响应。保存完成只更新已提交草稿对应的基线；用户在保存期间产生的新草稿必须保留，并继续处于编辑态。
