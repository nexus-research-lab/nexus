# Assistant 消息视图

- `assistant-message-model.ts`: 声明消费侧窄状态，并投影 Agent 作用域与紧凑/展开布局。
- `message-assistant-section.tsx`: 只组合助手外壳、头部、正文和统计。
- `assistant-message-content.tsx`: 按活动、direct 过程、归档过程、稳定最终回复、警告和权限顺序组合正文段；Room 把全部 pending request 固定交给唯一交互轨道，并在已有公区正文之后保留同一活动提示，DM 只保留 Composer-owned 请求的只读过程证据。
- `assistant-message-header.tsx`: 组合 32px、8px 圆角的头像与垂直居中的名称；展开态不混入时间和模型，紧凑态才显示它们，并保留外部动作与停止动作。
- `assistant-message-stats.tsx`: 负责结果统计、缓存后的模型、复制动作、流式游标和 Assistant 记忆引用入口，使用消费侧窄统计契约。
- `assistant-dm-tool-runs.tsx`: 只在 DM live direct 过程中把连续普通工具压缩成一个时间线块；未解析或新增长段强制展开，已解析段在叙事/final 恢复边界后默认折叠，生成文件仍保留在收起态。
- `assistant-process-callchain.tsx`: 独立管理过程折叠、过程内容和收起态生成文件。

本目录只消费控制器已经推导出的显示状态；不得重新排序消息、匹配权限或选择最终回复。
Assistant 入口按 header、permissions、direct、process、final、activity、footer 和 layout 消费状态；子视图只接收职责内切片，不索引上层聚合状态。
Room result 的 activity 仍由共享 `MessageActivityStatus` 呈现：无正文时占据原活动位，正文流式时跟随内容，正文暂歇而 execution 仍 active 时固定在正文尾部；三种阶段不得同时重复。
流式正文的稳定高度以 Assistant message identity 为边界：同一 turn 内只增不减，工具结果后的新 Assistant turn 即使 execution 仍 active 也必须重置，不得制造跨 Agent 卡片的大段空白。
DM 的 final 正文节点从 live 首字到 terminal backlog 排空必须保持同一组件位置；direct 过程与归档过程可以切换，但不得借此迁移或重挂正文。
`show_widget` 是答案本体，必须从首次流式输入到终态固定在 final surface，不得降级成工具过程卡。
DM live 工具段只能以首个 `tool_use.id` 作为 React 身份；流式 patch、结果与后续连续工具只能更新同一段，AskUserQuestion 和当前人工交互工具必须留在独立内容段。
权限只能由单一 owner 渲染：DM 与 Room 的完整响应面都由 Composer 原位替换输入壳；消息与 Thread 中的匹配工具只能显示等待状态，unmatched 请求不得再挂载正文控件。
Room execution 的 cancelled/error 终态必须同时投影到主 Feed 与 Thread 的未完成工具块；已有真实 `tool_result` 始终优先，只有缺失结果的工具才使用消息级终态覆盖。
`relevant_memories` 只在 Assistant 底部显示常驻引用入口，弹层仅展示脱敏摘要；不得恢复独立 system 消息或暴露记忆正文和绝对路径。
