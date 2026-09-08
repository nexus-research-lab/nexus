# Conversation 基础协议

本目录提供不属于具体页面 Feature 的会话标识、外部通道和跨消费者失效通知。

- `external-session.ts` 统一外部通道别名、标签、合成会话 ID 与会话投影。
- `agent-conversation-identity.ts` 将 DM、Room 与 Session 身份投影为稳定作用域键。
- `session-key.ts` 通过前缀规则表统一构造、校验和解析稳定 Session Key 协议。
- `room-directory-events.ts` 只广播 Room/DM 目录快照失效，不解释刷新策略。
- `workgraph-workflow-events.ts` 只定义命名工作图目录的跨消费者失效事件名；目录、Composer、全局通知与会话 transport 复用该合同，保存结果判断、刷新和 exact Session rebind 仍由各自消费者负责，不存在模型意图或命令旁路。
- `pending-permission-match.ts` 只按消息与工具调用身份精确匹配待处理权限，不解释展示状态。
- `configuration-secret-permission.ts` 将敏感配置草稿绑定到精确权限 request，并只选择服务端声明且完整填写的槽位。
- `message-protocol.ts` 在事件边界通过统一身份字段集合解码消息实体与流式载荷，并补齐信封提供的 Session 身份；消息恢复边界只接受 `durable`、`ephemeral` 与 `transient` 三种值；`workgraph_artifact` 只有带完整 preview 或 workflow 快照时才进入渲染。
- `live-stream-reveal.ts` 用本地 Symbol 连接实时 reducer 与 Markdown 首帧；标记不得进入持久协议，历史与恢复快照不得获得重播身份。
- `generative-ui.ts` 统一识别内建 `show_widget` 在各 runtime 中的包装名，供消息投影与虚拟列表估高共用。
- 仅由单一 Feature 消费的展示能力规则必须归还所属 Feature，不得以通用 helper 名义放入基础协议。
- 基础协议可以依赖 `types/` 和浏览器事件，不得依赖 `features/`、Store 或 React 视图。
- `types/` 只声明协议，不得反向调用本目录的解析函数。
