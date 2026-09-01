# 定时任务协议类型

- `task.ts` 持有任务定义、调度配置、对用户可见的 `deletion_state=deleting|review_required` 收尾/人工处理事实和 CRUD 参数；内部删除 token/时间不进入 Web 协议。
- `run.ts` 持有运行记录、状态和即时执行结果。
- `permission.ts` 持有 capability、持久审批请求、页面所见 job/run/revision 决策快照与决策结果。
- 未被前端消费的状态详情、事件和日报契约不在这里预声明。
