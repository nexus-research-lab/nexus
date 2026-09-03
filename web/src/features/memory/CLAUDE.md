# Memory 前端投影域

本目录只可视化和编辑 SDK workspace 中的文件式记忆，不拥有记忆生成、召回或整理逻辑。

## 职责边界

- `catalog/` 管理 Agent 级目录快照、筛选、选择、摘要投影和删除事务，所有状态绑定 `agentId`。
- `document/` 按作用域状态、正文资源、保存事务和视图拆分 `agentId:path` 文档。
- `use-scoped-memory-state.ts` 统一 Agent 与文档的作用域状态提交协议，旧异步结果只能提交到发起时的 scope。
- `agent-memory-view.tsx` 只编排资源状态和目录/正文工作面；当前 Agent 身份由上层上下文表达，不在记忆页重复渲染身份与摘要指标，刷新归属目录搜索工具区。
- `memory-utils.ts` 只保留跨目录共用的文档状态、Markdown frontmatter 与时间纯函数。

## 不变量

- Agent 快照、文档读取和保存结果必须匹配当前作用域，旧 Agent/路径不得回写。
- SDK 实时内容到达后必须使旧 HTTP 读取失效，编辑中的草稿不得被实时内容覆盖；异版 revision 必须进入用户决策，不自动合并。
- 保存期间继续编辑时，保存结果只更新基线内容，不覆盖新草稿或自动退出编辑。
- Memory 保存必须携带 exact 读取 revision；冲突不落盘，未知结果先读回对账，只有用户点击明确覆盖动作才能以最新 revision 再次提交。
- 删除只接受当前快照中 `memory/` 下的正文文档，必须确认后调用专用接口；`MEMORY.md` 索引不可删除，服务端负责同步清理索引行。删除结果未知时按 exact owner generation + Agent + path 锁定并先只读核对；条目仍在或核对失败都不得自动再次 DELETE，只有 `not_applied` 可安全重试，其他情况必须由用户显式开始并再次确认新意图。
- Memory UI 不读取或展示旧 `memory/sessions` 遗留结构。
- 文档类型的图标、色调和标签只由 Catalog 单一描述表定义，视图不得维护平行映射。
- 常态工作面以 8px 同色槽、轻微明度差和向左羽化阴影区分目录与正文，与 Room 右侧工作区保持同一种软分栏；不用重复摘要、文件路径或装饰性硬线制造层级。标题、提示、索引和正文统一使用 `nexus-memory-document-content` 阅读轴，目录激活态复用侧栏选择样式，仅异常和编辑状态可以使用强调边界。
- Memory 空目录和正文加载使用共享 Spinner 的 `lg`，Header 与按钮内瞬时状态使用 `xs/sm/md`；业务视图不得自行拼接旋转、颜色或 reduced-motion class。
