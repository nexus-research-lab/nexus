# 子智能体线程

本目录负责单个子智能体任务的 transcript 资源和只读展示装配。

## 职责边界

- `subagent-task-thread-model.ts` 只定义作用域、快照和纯展示投影。
- `use-subagent-task-thread-resource.ts` 独占 transcript 加载、请求代次和活动任务轮询。
- `use-subagent-task-thread.ts` 只组合资源和投影，不直接调用 API。
- `subagent-task-thread-view.tsx` 只消费窄视图模型，不解释请求或能力协议。

线程是只读执行记录，不提供 Composer、发送或停止动作；作用域切换后，旧请求不得写回当前任务。
