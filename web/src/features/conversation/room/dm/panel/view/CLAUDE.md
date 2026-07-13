# DM Chat Panel 视图

- `dm-chat-panel-view.tsx` 定义具体视图模型并只负责面板布局。
- 视图不得读取会话 Hook、调用领域 API 或重新推导 Feed 数据。
- 视图模型由消费者组件 Props 组成，不维护平行的宽接口。
