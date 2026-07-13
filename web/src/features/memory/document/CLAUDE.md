# Memory Document

本目录负责单个 `agentId:path` 记忆文档的读取、实时更新、编辑和保存。

## 边界

- `memory-document-model.ts` 以判别联合定义 Header 徽标、编辑操作和实时文件的 `ignore/apply/reload` 意图。
- `use-memory-document-state.ts` 通过共享作用域提交协议拥有文档状态，并保留保存响应与当前草稿的纯合并规则。
- `use-memory-document-resource.ts` 只执行 HTTP 读取和实时文件意图，旧请求不得覆盖新作用域。
- `use-memory-document-save.ts` 用不可变令牌固定保存请求的 Agent、路径和草稿，并只编排保存事务。
- `use-memory-document.ts` 组合资源和命令，向视图返回具体控制面。
- `index/` 独占 `MEMORY.md` 索引解析和导航视图。
- Panel 与 Header 分别渲染独占正文状态和操作栏，不自行组合业务状态。

SDK 实时内容优先于旧 HTTP 响应。保存完成只更新已提交草稿对应的基线；用户在保存期间产生的新草稿必须保留，并继续处于编辑态。
