# 子智能体任务域

本目录负责 DM 与 Room 共用的子智能体任务列表、线程资源和任务动作。

## 职责边界

- `subagent-task-model.ts` 只做服务端任务数据的归一化与纯派生。
- `use-subagent-tasks.ts` 管理来源级列表快照和活动任务轮询，快照必须绑定来源键。
- `use-subagent-task-thread.ts` 管理任务级 transcript、轮询和命令事务；资源错误与命令错误不得互相覆盖。
- `subagent-task-thread-view.tsx` 只消费视图所需的窄模型，不发起 API 请求或推导请求代次。
- `subagent-task-surface.tsx` 只负责任务列表与线程之间的页面选择。

## 不变量

- 所有异步结果必须同时匹配来源、任务和请求代次，旧作用域不得写回新页面。
- 发送与停止共用单一命令锁；同一任务不允许两个动作并发。
- 动作开放只依据服务端 capabilities，runtime kind 仅用于差异文案。
- 切换来源必须重建选择状态，不保留上一个来源的任务详情。
