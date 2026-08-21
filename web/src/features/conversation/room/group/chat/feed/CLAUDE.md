# Group Conversation Feed

## 数据流

`GroupConversationFeedProps` 按 `refs`、`source`、`renderer` 分组。普通列表与虚拟列表都通过 `resolveGroupConversationRound` 产生轮次状态，并由 `GroupConversationRound` 渲染。

## 约束

- 不在普通与虚拟列表中分别实现加载、Room 卡片或普通消息判断。
- Agent 身份目录、会话命令和运行阶段由面板投影完整提供，Feed 不维护假可选契约或空对象兼容分支。
- Room 公区的普通轮次不投影运行态占位，真实回复出现后再进入时间线。
- `memory_saved` 等 Agent 私有执行上下文只进入对应 Thread，不得在公区生成独立 system 节点；记忆召回保留模型 attachment，并仅在对应 Assistant 底部显示引用入口。
- 同一 root 的 Agent 节点与 root 保持相邻并沿用稳定 slot 顺序；状态完成不得把已展示节点移到其他 root 前后。
- 带精确 `agent_id + agent_round_id` 的 permission、slot 或 assistant message 任一先到，都必须直接建立同一个 `room-agent-round` 节点；不得先落 generic root 再搬入 Agent 节点。
- Feed 不读取侧栏未读 Store，不渲染“新消息”边界，也不在打开 Room 时自动定位历史节点；会话内只保留共享回到底部动作，避免未读收口事务与用户阅读锚点竞争。
- Feed 同时消费按 root 分组的 execution 首见锚点；权限提交成功后的 acknowledged 节点只保持原 shell 与活动反馈，不再携带可响应交互。
- 缺少 `agent_round_id` 的 legacy terminal 只有在 `parent_id` 精确命中同 Agent slot `msg_id` 时才能从 root 消费；不得按 Agent 唯一候选猜测执行归属。
- 任何阻塞 runtime、等待用户响应的请求，即使先于 Agent 消息或 slot 到达，或承载消息已经完成，也必须进入主 Room Composer；Feed 与 Thread 只保留只读等待证据。
- `group-conversation-height-model.ts` 只在共享消息估高上补入 slot-only 外壳；Composer-owned question/permission 不得在 Feed 重复预留高度。活跃 Agent 节点不得按完整未 reveal 正文预估高度。
- 最后 root 的全部连续 Agent 节点按真实高度进入 shared feed；新增 shell 必须立即推动 FOLLOW，禁止尾部 runway 或逐 Agent `min-height`。并行 live source 的负向校正只允许在共享父 Feed 暂存，并在整个 epoch 终态后统一释放。
- 导航优先定位已挂载 DOM；虚拟列表未挂载时才回退到索引滚动。
- 模型文件只做纯数据转换，不读取 Store、不触发副作用。
