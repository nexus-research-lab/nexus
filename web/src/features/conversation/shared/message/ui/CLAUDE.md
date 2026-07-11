# 消息共享 UI

- `message-avatar.tsx` 提供跨消息、Thread 与状态卡复用的身份头像和可选联络动作。
- `message-action-button.tsx` 提供消息头部复用的紧凑动作按钮与显式语气。
- `message-rail.tsx` 负责过程轨道的标签与正文布局。

共享 UI 必须有多个真实消费者；消息项私有状态展示和外壳留在 `item/`，禁止重新建立聚合导出文件。
