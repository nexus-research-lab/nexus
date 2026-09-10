# 消息项视图层

- `message-reading-layout.ts`: User/Assistant 共用的正文尺度与外侧节奏，以及各角色的头部、内容几何和 User 折叠阈值；这是阅读内容布局，不是 App chrome Typography 的第二套实现。
- `assistant/`: 助手身份、权限、过程调用链和正文编排。
- `content/`: Markdown/结构化内容投影、块分派、系统事件与时间线。
- `user/`: 用户正文、编辑状态和附件展示。
- `message-activity-status.tsx`: 以完整展示表渲染思考、工具、回复等活动图标、静态文案和语气；模型生成的即时旁白只替换这条活动行的通用文案，不创建第二个内容面。运行动效只能由固定占位的共享 `LoadingOrb` 承载；文案、图标和容器不做流光、pulse 或位移。

- 视图在消费侧声明窄输入契约，只读取控制器按职责分组的具体状态；不得重新推导消息顺序、最终回复、权限归属或运行阶段。
- Assistant 容器始终使用当前 DOM 的自然高度；本层不得通过 `ResizeObserver`、React state 或动态 `min-height` 另建一套高度真相源。流式增长节奏归 shared Markdown 内容调度器，消息领域投影位于相邻 `controller/`。
