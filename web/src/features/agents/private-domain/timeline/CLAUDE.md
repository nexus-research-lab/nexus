# Private Domain Timeline

- 时间线入口只组合 Header 与互斥的正文状态，不在 JSX 中重复判断加载、错误和空态。
- `agent-private-domain-timeline-model.ts` 统一投影事件方向、参与者名称、路由文案和相对时间。
- 所有展示文案与相对时间均由调用方传入的本地化上下文投影，事件原文不参与翻译。
- 事件方向与线程范围消费后端的封闭联合类型；未知值应在协议边界暴露，而不是由视图静默兜底。
- 时间线外壳使用公共 filled Panel；Header、名称、时间与路由使用 App Typography，密度不另建同值字号配方。
- 气泡视图仅消费已投影事件，方向和密度通过穷举映射决定布局；方向背景由事件视图拥有，用于区分 incoming/outgoing/self，圆角使用语义 Surface token。Markdown 正文保留阅读排版，不能因普通元数据收口改变正文或文件来源。
- 气泡正文在消费侧将 `sourceAgentId` 绑定为文件解析与图片预览能力，再注入共享 Markdown；全局 Agent 切换不能改写事件来源。
- 错误、未选择、空消息和事件列表互斥，新增状态时必须扩展状态表和对应视图。
- 时间线 Header 的增量读取使用共享 `md` muted Spinner；Header 不自行维护尺寸、颜色、旋转或 reduced-motion class。
