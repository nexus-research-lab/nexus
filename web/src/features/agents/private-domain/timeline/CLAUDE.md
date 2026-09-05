# Private Domain Timeline

- 时间线入口只组合 Header 与互斥的正文状态，不在 JSX 中重复判断加载、错误和空态。
- `agent-private-domain-timeline-model.ts` 统一投影事件方向、参与者名称、路由文案和相对时间。
- 所有展示文案与相对时间均由调用方传入的本地化上下文投影，事件原文不参与翻译。
- 事件方向与线程范围消费后端的封闭联合类型；未知值应在协议边界暴露，而不是由视图静默兜底。
- 气泡视图仅消费已投影事件，方向和密度通过穷举映射决定布局与外观。
- 气泡正文在消费侧将 `sourceAgentId` 绑定为文件解析与图片预览能力，再注入共享 Markdown；全局 Agent 切换不能改写事件来源。
- 错误、未选择、空消息和事件列表互斥，新增状态时必须扩展状态表和对应视图。
- 时间线 Header 的增量读取使用共享 `md` muted Spinner；Header 不自行维护尺寸、颜色、旋转或 reduced-motion class。
