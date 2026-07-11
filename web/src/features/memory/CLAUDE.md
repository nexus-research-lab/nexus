# Memory 前端投影域

本目录只可视化和编辑 SDK workspace 中的文件式记忆，不拥有记忆生成、召回或整理逻辑。

## 职责边界

- `use-agent-memory.ts` 管理 Agent 级目录快照、筛选和文档选择，所有状态绑定 `agentId`。
- `use-memory-document.ts` 管理 `agentId:path` 级内容、草稿、实时写入和保存事务。
- `agent-memory-view.tsx` 只编排摘要、目录和文档面板。
- `agent-memory-catalog.tsx` 只渲染筛选与文档目录。
- `memory-document-panel.tsx` 只消费文档控制器并渲染预览/编辑状态。
- `memory-utils.ts` 只保留 Markdown frontmatter、索引链接、时间与筛选纯函数。

## 不变量

- Agent 快照、文档读取和保存结果必须匹配当前作用域，旧 Agent/路径不得回写。
- SDK 实时内容到达后必须使旧 HTTP 读取失效，编辑中的草稿不得被实时内容覆盖。
- 保存期间继续编辑时，保存结果只更新基线内容，不覆盖新草稿或自动退出编辑。
- Memory UI 不读取或展示旧 `memory/sessions` 遗留结构。
