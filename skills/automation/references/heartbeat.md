# Heartbeat 与 wake

只在读取或修改 Agent heartbeat，或显式唤醒 Agent 时读取本文件。外部 IM round 不开放 heartbeat 配置读取；以 current contract 为准。

- `heartbeat` 是只读 query，返回当前可信范围的配置与状态。
- `set_heartbeat` 按 contract 可配置 enabled、`every_seconds`、`target_mode=none|last` 与 `ack_max_chars`。Heartbeat 是允许调度抖动的低频自检机制，不保证用户在精确时间收到结果。
- `wake` 使用 `mode=now|next-heartbeat` 和可选 text。now 表示尽快唤醒；next-heartbeat 只登记到下一次周期，不创建准确时间任务。

需要精确时间、一次性提醒、周期报表、独立 permission/context 快照或结果投递时，创建 scheduled task。只需要 Agent 宽松检查积累事件、维护状态或在下一 heartbeat 消费提示时使用 heartbeat。

设置前 inspect 当前配置，mutation 后重新查询验证。不要用 wake 模拟 scheduled task，也不要通过高频 heartbeat 规避任务 overlap、权限或投递模型。
