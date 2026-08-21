# Assistant 消息视图

- `assistant-message-model.ts`: 声明消费侧窄状态，并投影 Agent 作用域与紧凑/展开布局。
- `message-assistant-section.tsx`: 只组合助手外壳、头部、正文和统计。
- `assistant-message-content.tsx`: 按活动、direct 过程、归档过程、稳定最终回复、警告和权限顺序组合正文段；Room 把全部 pending request 固定交给唯一交互轨道，并在已有公区正文之后保留同一活动提示，DM 只保留 Composer-owned 请求的只读过程证据。
- `assistant-message-header.tsx`: 组合 32px、8px 圆角的头像与垂直居中的名称；宿主 handoff reply 以不可点击的“回应 @Agent”身份 chip 展示，不注入正文 mention 或动作；展开态不混入时间和模型，紧凑态才显示它们，并保留外部动作与停止动作。
- `assistant-message-stats.tsx`: 只渲染控制器已判定可见的结果统计、模型、复制动作、Assistant 记忆引用入口，以及附着在最终回复下方且省略未知耗时/token 的 Goal 完成收据；内部 goal/round ID 永不展示，也不接收运行态。
- `goal-completion-receipt.ts`: 负责 Goal 完成收据的可见字段选择、耗时格式化与本地化拼装；未知值不生成任何占位文案。
- `assistant-dm-tool-runs.tsx`: 在 DM/Room live direct 过程中把中间旁白、thinking 与连续普通工具压缩成一个默认收起、持续更新的单行时间线块；最终回复由独立 final surface 保持可见，ToolUseSummary 按精确工具 ID 替换执行段标题，active 也允许点击展开完整过程，生成文件仍保留在收起态；自然 summary 不追加冗余状态词，终态则改为中性的“执行过程”审计入口，不再用工具次数冒充进展。
- `assistant-process-callchain.tsx`: 独立管理过程折叠、过程内容和收起态生成文件。

本目录只消费控制器已经推导出的显示状态；不得重新排序消息、匹配权限或选择最终回复。
Assistant 入口按 header、permissions、direct、process、final、activity、footer 和 layout 消费状态；子视图只接收职责内切片，不索引上层聚合状态。
Room result 的普通工具活动由共享折叠工具行承载；没有工具内容时才由 `MessageActivityStatus` 占据原活动位，正文流式时跟随内容，三种阶段不得同时重复。
流式正文只保持内容身份稳定，不缓存历史最大高度；同一 turn 的 Markdown 前缀按共享公平池追加，工具、活动与 final surface 切换后容器立即服从当前 intrinsic layout，不得用空白偿还先前高度。
DM 的 final 正文节点从 live 首字到 terminal backlog 排空必须保持同一组件位置；direct 过程与归档过程可以切换，但不得借此迁移或重挂正文。
`show_widget` 是答案本体，必须从首次流式输入到终态固定在 final surface，不得降级成工具过程卡。
DM/Room live 工具段只能以首个 `tool_use.id` 作为 React 身份；流式 patch、结果与后续连续工具只能更新同一段，AskUserQuestion、当前人工交互工具和 `show_widget` 必须留在独立内容段。
权限只能由单一 owner 渲染：DM 与 Room 的完整响应面都由 Composer 原位替换输入壳；消息与 Thread 中的匹配工具只能显示等待状态，unmatched 请求不得再挂载正文控件。
Room execution 的 cancelled/error 终态必须同时投影到主 Feed 与 Thread 的未完成工具块；已有真实 `tool_result` 始终优先，只有缺失结果的工具才使用消息级终态覆盖。
`relevant_memories` 只在 Assistant 底部显示常驻引用入口，弹层仅展示脱敏摘要；不得恢复独立 system 消息或暴露记忆正文和绝对路径。
