# 子智能体任务域

本目录负责 DM 与 Room 共用的子智能体任务列表、线程资源和任务动作。

## 职责边界

- `subagent-task-model.ts` 只做服务端任务数据的归一化与纯派生；实例标题优先使用模型拉起任务时给出的名称/短描述，`agent_type` 仅作缺省回退；消息任务入口只能通过精确 `tool_use_id`（旧记录允许同值 `task_id`）解析详情，不按标题或时间猜测。列表、线程和消息入口头像统一复用 Skill 数学曲线生成器，以 `tool_use_id` 为稳定种子，旧记录回退 `task_id`。
- `subagent-task-list-model.ts` 按可选 `host_agent_id` 过滤当前 Session 的全部任务，再单次分组并排序，同时投影加载与 runtime 支持状态。
- `use-scoped-resource.ts` 统一来源/任务作用域、请求代次和原子快照提交，不解释业务状态。
- `use-subagent-tasks.ts` 只管理来源级列表加载和面板可见期间的任务发现轮询；首次空列表不得阻止后续 task 被发现。
- `thread/` 按纯投影、transcript 资源、mutation controller 和视图拆分单任务线程；服务端返回的独立 Agent transcript 必须复用普通对话消息投影，隐藏父 Agent 下发的 user 任务提示，只保留子 Agent 的思考、回复、工具调用与结果。控制项严格由 task capability 决定：运行中可停止，nxs 可补充指令或沿同一 task 恢复；unsupported 必须解释，不能渲染无效动作。
- `subagent-task-surface.tsx` 只负责任务列表与线程之间的页面选择，并接收上游提供的调用者过滤条件、精确 ToolUse 导航请求和头部插槽；同一次请求只自动进入一次，用户返回列表后轮询刷新不得再次强制打开；不得反向依赖 Room 成员组件。

## 不变量

- 所有异步资源统一通过作用域资源原语提交，旧作用域不得写回新页面。
- capabilities 归一化由统一字段表驱动；新增能力时不得在返回对象中复制逐字段回退。
- 切换来源必须重建选择状态，不保留上一个来源的任务详情。
