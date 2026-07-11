# Conversation Timeline

- `timeline-model.ts` 定义时间线投影、按根轮次分组和纯构建函数，是 feed 与 navigator 的共同数据源。
- `use-conversation-timeline.ts` 只负责分组结果的 React 记忆化，不承载加载协议。
- `round-scroll.ts` 集中滚动定位与恢复协议；消费者不得复制 DOM 属性或偏移常量。
- `use-visible-window-loader.ts` 以会话代次和请求编号识别在途加载，旧会话完成不得修改新会话状态。
- 可见窗口加载失败采用有限重试；失败轮次不能在当前会话内被永久忽略。
- 历史追加、索引窗口加载和导航定位共享同一时间线模型，不得各自派生轮次顺序。
