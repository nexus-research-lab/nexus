# Room Round

- `round-agent-model.ts` 消费 canonical timeline 已投影的公区证据，负责一轮内按 `agent_round_id` 对齐 Agent 消息、结果、占位槽与带精确执行身份的 permission；任一证据先到都建立同一 entry，同 Agent 的多次执行不得坍缩，不再自行解释私域可见性。
- Agent entry 按当前 Session 内的 canonical 展示锚点排序；历史/初始 snapshot 先用持久 `display_order` 或 message/slot timestamp + index 播种顺序，实时 permission、slot、stream、status 或 message fallback 后续首次出现时沿用同一时间尺度只追加。持久 message 的显式 `display_order` 是唯一可在实时事件抢先到达时回填更早 entry 或校正同一 execution 的权威证据；legacy fallback 与其他后到证据只能补齐执行而不得换位。完成时间只用于 header 语义。
- permission 成功响应后移除交互请求，但保留 acknowledged 非交互 execution shell，直到 slot、message 或 terminal 状态接管；Session 切换与 root 重写必须精确清理。
- `round-thread-model.ts` 负责从根轮次投影精确 Agent 执行轮的 Thread 消息。
- Room 主 Feed 与 Thread 必须消费同一 Agent 聚合模型，不各自推导执行状态。
- 结果状态映射与消息状态优先级由数据表定义；合成 result 只在 canonical assistant 缺席时保留。
- 权威 lifecycle terminal 是 Agent entry 状态的单调上界；迟到的 active slot、stream 与精确 permission 都不能复活已收口 execution，冲突终态按 error、cancelled、done 的优先级合并。
- 权威 lifecycle active 同样覆盖尚无 `result_summary` 的 Assistant turn 完成态；Thread 仍执行时，主 Feed 必须继续显示思考/回复活动提示，不能把 `message_stop` 或 `is_complete` 当成整个 `agent_round` 结束。
- 重连后同一根轮次与 Agent 出现更新的 execution 时，较早且未收口的 execution 单调转为 cancelled；分页拆出的公区节点继续携带该终态，历史 entry 与顺序保留，不能让旧 Stop 控件与新执行并存。
- 本目录只放纯模型，不读取 Store、不调用 API、不持有 React 状态。
