# 定时任务协议类型

- `task.ts` 持有任务定义、调度配置和 CRUD 参数。
- `run.ts` 持有运行记录、状态和即时执行结果。
- `permission.ts` 持有 capability、持久审批请求、页面所见 job/run/revision 决策快照与决策结果。
- 未被前端消费的状态详情、事件和日报契约不在这里预声明。
