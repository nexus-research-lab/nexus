# 消息项视图层

- `assistant/`: 助手身份、权限、过程调用链和正文编排。
- `content/`: Markdown/结构化内容投影、块分派、系统事件与时间线。
- `user/`: 用户正文、编辑状态和附件展示。
- `message-item-streaming-layout.ts`: 流式阶段的高度测量与稳定布局。

- 视图消费 `MessageItemState`，不得重新推导消息顺序、最终回复、权限归属或运行阶段。
- 流式高度测量属于视觉布局行为，保留在本层；消息领域投影位于相邻 `controller/`。
