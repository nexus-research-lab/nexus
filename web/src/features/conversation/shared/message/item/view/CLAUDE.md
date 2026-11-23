# 消息项视图层

- `message-reading-layout.ts`: User/Assistant 共用的正文尺度与外侧节奏，以及各角色的头部、内容几何和 User 折叠阈值；这是阅读内容布局，不是 App chrome Typography 的第二套实现。
- `assistant/`: 助手身份、权限、过程调用链和正文编排。
- `content/`: Markdown/结构化内容投影、块分派、系统事件与时间线。
- `user/`: 用户正文、编辑状态和附件展示。
- `message-activity-status.tsx`: 以完整展示表（图标、文案、语气、是否计时、playful/literal 口吻）渲染活动行。有轮次计时源时，运行中阶段改为“前置 `LoadingOrb` + 文案 + 已用时长”：发送/思考/回复这类泛化阶段显示轮换趣味动词，工具、压缩保留真实阶段文案，等待确认/输入保持图标 + 静态文案且不计时；模型即时旁白优先于趣味动词，不创建第二个内容面。没有计时源（如 Team 远程等待行）时退回静态阶段文案。文案、图标和容器不做流光、pulse 或位移；阶段切换与换词只允许 `ui-status-swap` 一次性淡入交接。
- `message-activity-clock.tsx`: MessageItem 以本轮最早消息时间提供计时源；全部活动行共用一个按需启停的秒级 ticker，动词按轮次身份与已用时长确定性选择，活动行在独立行、工具段与尾随位置之间迁移时不重置计时、不换词。

- 视图在消费侧声明窄输入契约，只读取控制器按职责分组的具体状态；不得重新推导消息顺序、最终回复、权限归属或运行阶段。
- Assistant 容器始终使用当前 DOM 的自然高度；本层不得通过 `ResizeObserver`、React state 或动态 `min-height` 另建一套高度真相源。流式增长节奏归 shared Markdown 内容调度器，消息领域投影位于相邻 `controller/`。
