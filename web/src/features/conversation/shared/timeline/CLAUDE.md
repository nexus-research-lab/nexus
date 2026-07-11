# Conversation Timeline

- `timeline-model.ts` 定义时间线投影、按根轮次分组和纯构建函数，是 feed 与 navigator 的共同数据源。
- `use-conversation-timeline.ts` 只负责分组结果的 React 记忆化，不承载加载协议。
- `scroll/` 集中自动跟随、历史锚点、局部展开锚点、动画和轮次 DOM 导航协议。
- `window-loader/` 分离可见候选选择、请求/重试运行时和 React 调度；旧会话完成不得修改新会话状态。
- 可见窗口加载失败采用有限重试；失败轮次不能在当前会话内被永久忽略。
- 历史追加、索引窗口加载和导航定位共享同一时间线模型，不得各自派生轮次顺序。
