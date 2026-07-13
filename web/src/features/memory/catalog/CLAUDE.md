# Memory Catalog

本目录负责 Agent 记忆目录的请求、筛选、选择和摘要投影，不读取文档正文。

## 边界

- `memory-catalog-model.ts` 保存筛选定义和纯匹配规则，并直接产出分区、行选择、空态与截断投影。
- `memory-catalog-presentation.ts` 以单一描述表定义文档图标、色调和标签。
- `use-agent-memory.ts` 通过共享作用域提交协议绑定 Agent 请求代次，并提供按目录、文档和摘要分组的控制面。
- `agent-memory-catalog.tsx` 只遍历 Catalog 投影，不重新解释快照、筛选或文档类型规则。

目录请求结果必须匹配当前 `agentId`。选择路径只能指向当前快照中的文档，视图不得自行修正失效选择。
