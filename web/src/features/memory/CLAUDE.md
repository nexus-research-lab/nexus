# Memory 前端投影域

本目录只可视化和编辑 SDK workspace 中的文件式记忆，不拥有记忆生成、召回或整理逻辑。

## 职责边界

- `catalog/` 管理 Agent 级目录快照、筛选、选择和摘要投影，所有状态绑定 `agentId`。
- `document/` 按作用域状态、正文资源、保存事务和视图拆分 `agentId:path` 文档。
- `use-scoped-memory-state.ts` 统一 Agent 与文档的作用域状态提交协议，旧异步结果只能提交到发起时的 scope。
- `agent-memory-view.tsx` 只编排摘要和内容区；Agent 身份、指标、资源状态各自拥有窄视图。
- `memory-utils.ts` 只保留跨目录共用的文档状态、Markdown frontmatter、时间与尺寸纯函数。

## 不变量

- Agent 快照、文档读取和保存结果必须匹配当前作用域，旧 Agent/路径不得回写。
- SDK 实时内容到达后必须使旧 HTTP 读取失效，编辑中的草稿不得被实时内容覆盖。
- 保存期间继续编辑时，保存结果只更新基线内容，不覆盖新草稿或自动退出编辑。
- Memory UI 不读取或展示旧 `memory/sessions` 遗留结构。
- 文档类型的图标、色调和标签只由 Catalog 单一描述表定义，视图不得维护平行映射。
