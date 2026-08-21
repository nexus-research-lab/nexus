# DM Chat Panel 视图

- `dm-chat-panel-view.tsx` 定义具体视图模型并只负责面板布局。
- DM 有 pending interaction 时，视图把确认组件作为 Composer 输入壳的互斥内容传入；不得追加输入框上方浮层。
- 视图不得读取会话 Hook、调用领域 API 或重新推导 Feed 数据。
- 视图模型由消费者组件 Props 组成，不维护平行的宽接口。
- 嵌入模式只改变外壳可见性，不分叉消息、Tool、流式状态或 Composer 的渲染实现。
