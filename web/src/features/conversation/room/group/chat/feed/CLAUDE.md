# Group Conversation Feed

## 数据流

`GroupConversationFeedProps` 按 `refs`、`source`、`renderer` 分组。普通列表与虚拟列表都通过 `resolveGroupConversationRound` 产生轮次状态，并由 `GroupConversationRound` 渲染。

## 约束

- 不在普通与虚拟列表中分别实现加载、Room 卡片或普通消息判断。
- Agent 身份目录、会话命令和运行阶段由面板投影完整提供，Feed 不维护假可选契约或空对象兼容分支。
- Room 公区的普通轮次不投影运行态占位，真实回复出现后再进入时间线。
- 同一 root 的 Agent 节点与 root 保持相邻并沿用稳定 slot 顺序；状态完成不得把已展示节点移到其他 root 前后。
- 带精确 `agent_id + agent_round_id` 的 permission、slot 或 assistant message 任一先到，都必须直接建立同一个 `room-agent-round` 节点；不得先落 generic root 再搬入 Agent 节点。
- Feed 同时消费按 root 分组的 execution 首见锚点；权限提交成功后的 acknowledged 节点只保持原 shell 与活动反馈，不再携带可响应交互。
- 缺少 `agent_round_id` 的 legacy terminal 只有在 `parent_id` 精确命中同 Agent slot `msg_id` 时才能从 root 消费；不得按 Agent 唯一候选猜测执行归属。
- 任何阻塞 runtime、等待用户响应的请求，即使先于 Agent 消息或 slot 到达，或承载消息已经完成，也必须在主 Room 暴露完整交互入口，不得只保留在 Thread。
- `group-conversation-height-model.ts` 在共享消息估高上补入 slot-only 外壳和 pending question/permission 高度；已匹配工具由独立交互轨道接管时必须先扣除共享 tool_use 基础估高，禁止重复计算。
- 最后 root 的全部连续 Agent 节点按真实高度进入 shared feed；新增 shell 必须立即推动 FOLLOW，禁止尾部 runway 或逐 Agent `min-height`。
- 导航优先定位已挂载 DOM；虚拟列表未挂载时才回退到索引滚动。
- 模型文件只做纯数据转换，不读取 Store、不触发副作用。
