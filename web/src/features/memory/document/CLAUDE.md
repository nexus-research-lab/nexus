# Memory Document

本目录负责单个 `agentId:path` 记忆文档的读取、实时更新、编辑和保存。

## 边界

- `memory-document-model.ts` 以判别联合定义 Header 编辑操作和实时文件的 `ignore/consume/apply/reload/conflict` 意图。
- `use-memory-document-state.ts` 通过共享作用域提交协议拥有文档状态，并保留保存响应与当前草稿的纯合并规则。
- `use-memory-document-resource.ts` 只执行 HTTP 读取和实时文件意图，旧请求不得覆盖新作用域。
- `use-memory-document-save.ts` 用不可变令牌固定保存请求的 Agent、路径、草稿和读取 revision；未知结果先精确读回对账，冲突只能由用户明确选择后覆盖。
- `use-memory-document.ts` 组合资源和命令，向视图返回具体控制面。
- `index/` 独占 `MEMORY.md` 索引解析和导航视图。
- Panel 与 Header 分别渲染独占正文状态和操作栏，不自行组合业务状态。
- Header 只展示统一投影的可读摘要标题、更新时间、运行时状态和真实操作，真实路径只作为标题悬浮说明；正文记忆提供确认删除，索引不提供删除。标题、提示和正文共用阅读轴，索引导航只展示标题与摘要，路径仅作为内部导航目标。
- Memory 过期提示使用 `UiInlineNotice`，读取、保存和冲突仍按各自资源状态呈现；Panel 只判断事实，不复制提示条的圆角、背景或字号。
- Header 的运行时写入、保存和删除，以及正文初始加载统一使用共享 Spinner 尺寸与动效配方；非加载态图标保持原动作图标，不用旋转表达普通刷新。

SDK 实时内容优先于旧 HTTP 响应。保存完成只更新已提交草稿对应的基线；用户在保存期间产生的新草稿必须保留，并继续处于编辑态。

读取正文的 revision 是 Memory 保存的必要前提。Agent 或另一页在编辑期间更新同一文件时，必须保留草稿并进入对照决策；断线等未知写结果必须先读回 exact Agent + path 对账，不得盲目重放。
