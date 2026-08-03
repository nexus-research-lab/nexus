# 内容块视图

- `content-renderer.tsx`: 只区分 Markdown 与结构化内容入口。
- `content-renderer-contract.ts`: 定义入口与结构化编排器共同消费的窄属性契约，包括未返回 `tool_result` 时由 execution terminal evidence 提供的 stopped/error 收口。
- `structured-content-renderer.tsx`: 建立一次内容投影并编排块视图、时间线和流式活动状态。
- `content-renderer-model.ts`: 建立 toolUse/result、任务进度、已消费块索引与 live 文本挂载判定。
- `content-block-view.tsx`: 通过穷尽注册表分派 ContentBlock，并拥有空节点和时间线框架；live 空文本必须先挂载 Markdown 身份，让首批正文进入平滑 backlog，静态空文本仍不占布局。
- `content-tool-block.tsx`: 所有消息内工具统一投影为静态 ToolBlock 证据；`AskUserQuestion` 无论处于 pending、历史完成、恢复或未匹配状态，都不得在 DM、Room、Thread 或过程展开中重新挂载选项树，唯一可操作入口属于 Composer。
- `content-system-event.tsx`: 渲染系统事件与 API 重试倒计时。
- `content-renderer-timeline.tsx`: 测量并对齐时间线圆点。

内容数组只在纯投影层建立关联；具体块视图不得再次扫描整轮内容或猜测工具归属。
新增内容块类型必须同时进入穷尽渲染注册表，禁止在编排器中追加类型分支。
内容投影向相邻 `activity/` 提供已消费块、已结束工具和隐藏工具集合；活动领域不得反向依赖本目录的视图模型。
DOM 锚点测量和系统事件样式属于具体视图，不得回流到消息领域模型。
