# Agent 私域

- `agent-private-domain-thread-model.ts` 统一投影线程标题、加载/空/就绪状态、密度样式、Scope 和元数据。
- `agent-private-domain-thread-list.tsx` 只按联合状态渲染列表，并使用固定 Scope 图标表，不重新解释线程字段。
- `agent-private-domain-view.tsx` 负责工具栏、列表和时间线装配；时间线内部规则归 `timeline/`。
