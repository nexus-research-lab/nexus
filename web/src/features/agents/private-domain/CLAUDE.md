# Agent 私域

- `agent-private-domain-thread-model.ts` 统一投影线程标题、加载/空/就绪状态、密度、Scope、最近时间和单行摘要；目录不恢复房间名、消息数等重复元数据，也不持有交互行的 active/hover/focus 样式。
- 线程与时间线模型必须显式接收当前语言和翻译函数，禁止以中文默认值掩盖未接入国际化的界面状态。
- `agent-private-domain-thread-list.tsx` 只按联合状态渲染列表，固定复用 `UiListRow activeTone="sidebar"` 与 Scope 图标表，不重新解释线程字段或手写按钮壳。
- `agent-private-domain-view.tsx` 负责工具栏、列表和时间线装配；完整详情复用记忆页的 240–288px 紧凑目录与 8px 软分栏，右侧正文保持 920px 阅读轴，Room 预览继续使用自身紧凑几何；时间线内部规则归 `timeline/`，分栏材质归 `agent-private-domain.css`。
- 私域线程目录初始加载使用共享 `lg` muted Spinner，工具栏刷新使用 `sm` muted Spinner；视图不得自行维护旋转或 reduced-motion class。
