# 子智能体线程

本目录负责单个子智能体任务的 transcript 资源和 capability-driven 控制装配。

## 职责边界

- `subagent-task-thread-model.ts` 只定义作用域、快照和纯展示投影。
- `use-subagent-task-thread-resource.ts` 独占 transcript 加载、请求代次和精确 task 的实时失效刷新。
- `use-subagent-task-actions.ts` 独占精确 task 的 stop/send/resume mutation、请求代次、取消与反馈。
- `use-subagent-task-thread.ts` 只组合资源和投影，不直接调用 API。
- `subagent-task-thread-view.tsx` 只消费窄视图模型，不解释请求或能力协议。

线程不伪装成普通 Composer；只在服务端 capability 允许时提供停止和“补充指令/继续任务”，停止必须二次确认。作用域切换后，旧查询或 mutation 均不得写回当前任务。

线程初始加载使用共享 `md` Spinner，停止、补充指令和继续任务使用 `sm`；视图不得自行维护颜色、旋转或 reduced-motion class。
