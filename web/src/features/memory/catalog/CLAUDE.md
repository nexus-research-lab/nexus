# Memory Catalog

本目录负责 Agent 记忆目录的请求、筛选、选择和目录投影，不读取文档正文。

## 边界

- `memory-catalog-model.ts` 保存筛选定义和纯匹配规则，并直接产出分区、行选择、空态与截断投影。
- `memory-catalog-presentation.ts` 以单一映射定义文档类型图标和面向用户的主显示标题；有真实摘要时用摘要作为主标题，目录统一使用中性色调，类型不是状态，不引入竞争色。
- `memory-deletion-recovery.ts` 只根据服务端 effect 与 exact path 目录核对结果决定安全重试、继续核对或显式新删除意图。
- `memory-deletion-issue-notice.tsx` 持久展示每个未解决删除的 Problem / Impact / Recovery，不解释错误正文。
- `use-agent-memory.ts` 通过共享作用域提交协议绑定 owner generation、Agent、path 和命令代次；删除提交与目录刷新分阶段，完整 DELETE 成功后只允许重刷目录。
- `agent-memory-catalog.tsx` 只遍历 Catalog 投影，不重新解释快照、筛选或文档类型规则；搜索、刷新与类型菜单共用一行工具区。目录项复用 `UiListRow density="dense"`，以可读摘要为主标题，它与文档名不同时保留文档名作为次级身份，完整路径只参与搜索与悬停说明。
- 刷新按钮仅在真实刷新期间使用共享 `sm` Spinner 配方；静止状态继续显示普通刷新图标，Catalog 不拥有旋转或 reduced-motion 样式。

目录请求结果必须匹配当前 owner generation 与 `agentId`。选择路径只能指向当前快照中的文档；没有仍然有效的选择时默认打开最近的正文记忆，索引保留为显式入口。普通刷新可以更新 stale 快照，但不得解除某个 path 的未知删除；只有显式核对确认条目缺失才能收口，截断目录还必须用 exact path GET 补证。
