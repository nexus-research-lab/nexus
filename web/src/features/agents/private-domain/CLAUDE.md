# Agent 私域

- `agent-private-domain-thread-model.ts` 只投影线程标题、加载/空/就绪状态、Scope、最近时间和单行摘要等数据；不得返回 class、Typography 或密度。`agent-private-domain-thread-layout.ts` 单独拥有 compact/regular 容器、行密度和文字 recipe；目录不恢复房间名、消息数等重复元数据，也不持有交互行的 active/hover/focus 样式。
- 线程与时间线模型必须显式接收当前语言和翻译函数，禁止以中文默认值掩盖未接入国际化的界面状态。
- 线程标题与头像的缺失/空白姓名复用 `lib/agent-display-name.ts`；头像的可访问名称由消费侧提供，不再独立用 ID 兜底。线程列表向共享 Markdown summary 注入本地化公式标记，不在目录中排版公式；全文继续由时间线呈现。
- `agent-private-domain-thread-list.tsx` 只按联合状态渲染列表，固定复用 `UiListRow activeTone="sidebar"` 与 Scope 图标表，不重新解释线程字段或手写按钮壳。
- `agent-private-domain-view.tsx` 负责工具栏、列表和时间线装配；完整详情复用记忆页的 240–288px 紧凑目录与 8px 软分栏，右侧正文保持 920px 阅读轴，Room 预览继续使用自身紧凑几何；时间线内部规则归 `timeline/`，分栏几何归 `agent-private-domain.css`；预览目录与两种密度的时间线均使用 `UiPanel variant="filled"`，不得恢复私有阅读面背景或分栏阴影。
- 工具栏标题和计数使用 App Typography。私域头像仍由 `UiAgentAvatar` 渲染；多参与者的叠放与溢出计数属于 `agent-private-domain-avatar.tsx` 的身份图形几何，不承担按钮或普通元数据职责。
- 私域线程目录初始加载使用共享 `lg` muted Spinner，工具栏刷新使用 `sm` muted Spinner；视图不得自行维护旋转或 reduced-motion class。

线程与消息读取分别持有单调请求代次；同 scope 重读、A→B→A 和卸载都使旧结果失效，过期成功/失败/finally 不得覆盖新快照或清除新 loading。初始线程加载具名 status，刷新和时间线明确声明 busy。

- DM简介联络按Agent读取跨会话记录，不使用当前人机DM的Room/Conversation过滤。群聊简介继续保留精确Room/Conversation；未指定Room时预览也不将扫描范围压到一个Room。
