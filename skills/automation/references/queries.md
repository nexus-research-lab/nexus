# Automation 查询

只在使用 `list`、`get`、`runs`、`events`、`report` 或读取当前 scheduled run 时读取本文件。operation 和字段以 `nexus.automation_read` 当前 schema 为准；query 不经过 plan/apply。

- `list`：按 schema 可用的 query、agent、active/deleted、enabled 与 limit 过滤。外部 IM 的空查询默认只覆盖当前可信会话。
- `get`：使用 exact `job_id` 或能唯一定位的 `query`；scheduled run 可省略 job locator 使用宿主绑定任务。按需限制 runs/events 数量。
- `runs` / `events`：使用 exact job 或唯一 query 读取执行、审批、恢复和投递事实；支持已删除任务历史。
- `report`：按 schema 使用 date、timezone、agent、job 或唯一 query；不要从聊天正文构造底层 Session route。
修改、删除、立即运行或重投递前，先用查询定位唯一任务并核对 `job_id`、`configuration_version`、active run 与健康状态。多个候选时停止并让用户明确目标，不按名称相似度猜测。

后台 scheduled run 只允许查询宿主绑定的当前 `job_id/run_id`。不要通过显式 locator、agent 字段或跨会话 query 扩大范围。
