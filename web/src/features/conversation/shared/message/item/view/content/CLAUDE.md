# 内容块视图

- `content-renderer.tsx`: 只区分 Markdown 与结构化内容入口。
- `content-renderer-contract.ts`: 定义入口与结构化编排器共同消费的窄属性契约，包括未返回 `tool_result` 时由 execution terminal evidence 提供的 stopped/error 收口，以及外层状态 owner 禁止尾随活动的显式开关。
- `structured-content-renderer.tsx`: 建立一次内容投影并编排块视图、时间线和流式活动状态；消息级活动只在没有可见叶子块或外层过程栏持有状态时作为 fallback，运行中/等待中的 ToolBlock 或 ToolUseSummary 执行栏已展示自己的状态后不得再叠加同义活动行。普通 Thought/工具活动的过程分段只由相邻 `process/dm-tool-run-segments.ts` 建立，本层不再维护第二套分组。非时间线中的连续 Agent/Task 启动只在存在子智能体导航命令时合并为可换行的紧凑任务入口组，待权限工具仍保持独立。
- `content-renderer-model.ts`: 建立 toolUse/result、任务进度、已消费块索引与 live 文本挂载判定。
- `content-block-view.tsx`: 通过穷尽注册表分派 ContentBlock，并拥有空节点和时间线框架；live 空文本必须先挂载 Markdown 身份，让首批正文进入平滑 backlog，静态空文本仍不占布局；`progress_update` 是非渲染协议块，由消息活动投影读取，不得包装成正文或过程摘要。
- `content-tool-block.tsx`: 普通消息内工具统一投影为静态 ToolBlock 证据；内建 `show_widget` 是唯一生成式 UI 视图分支。`AskUserQuestion` 无论处于 pending、历史完成、恢复或未匹配状态，都不得在 DM、Room、Thread 或过程展开中重新挂载选项树，唯一可操作入口属于 Composer。
- `content-system-event.tsx`: 渲染系统事件与 API 重试倒计时；长期记忆保存事件使用与工具、Thought 相同的单行活动记录，不再恢复旧双行短轨。
- `content-renderer-timeline.tsx`: 测量并对齐时间线圆点，主轨与内容列保持紧凑间距。

内容数组只在纯投影层建立关联；具体块视图不得再次扫描整轮内容或猜测工具归属。
新增内容块类型必须同时进入穷尽渲染注册表，禁止在编排器中追加类型分支。
内容投影向相邻 `activity/` 提供已消费块、已结束工具和隐藏工具集合；活动领域不得反向依赖本目录的视图模型。
状态所有权按 `Goal lifecycle/activity -> Agent execution -> reply/tool leaf` 逐层细化；同一时刻只由最深可见层展示瞬时状态，上层不得重复解释下层已经可见的状态。
DOM 锚点测量和系统事件样式属于具体视图，不得回流到消息领域模型。
