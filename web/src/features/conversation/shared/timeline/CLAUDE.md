# Conversation Timeline

- `timeline-model.ts` 定义时间线投影、消息/slot/permission/Room execution 锚点的根轮次分组和可见内容规则，是 feed 与 navigator 的共同数据源；隐藏消息、私域 Room execution 证据及其 live round 只在这里过滤一次。
- `use-conversation-timeline.ts` 只负责分组结果的 React 记忆化，不承载加载协议；分组经 `stable-group-map.ts` 结构共享，逐元素同一引用的轮次跨流式帧保持数组身份，Room 节点投影复用同一入口。
- `scroll/` 集中自动跟随、历史锚点、局部展开锚点、动画和轮次 DOM 导航协议；所有视口位移经 `scroll/scroll-controller.ts` 以具名写入者提交。
- `window-loader/` 分离连续窗口的边界决策和 React 调度；旧会话完成不得修改新会话状态。
- 驻留窗口是一段连续 round 区间加实时尾部；窗口只在两端扩展并从相反端淘汰，Feed 不渲染未驻留 round 的占位节点。
- 窗口加载失败或无进展只进入短冷却；任何一端不能在当前会话内被永久关闭，被淘汰的轮次靠近边界时重新具备加载资格。
- 历史追加、连续窗口加载和导航定位共享同一时间线模型，不得各自派生轮次顺序。
- `buildResidentRoundIds` 用完整索引确定顺序、只输出正文已驻留的轮次；不保留仅供旧测试使用的转发入口。
- 主会话只使用连续窗口加载器；旧式 prepend loader 仅服务 contacts/team 的无索引会话，且滚动触发必须来自 READING，初始 FOLLOW 与会话加载中的程序化滚动不得创建历史锚点事务。
