# 子智能体线程

本目录负责单个子智能体任务的 transcript 资源和 capability-driven 控制装配。

## 职责边界

- `subagent-task-thread-model.ts` 只定义作用域、快照和纯展示投影。
- `use-subagent-task-thread-resource.ts` 独占 transcript 加载、请求代次和精确 task 的实时失效刷新。
- `use-subagent-task-actions.ts` 独占精确 task 的 stop/send/resume mutation、请求代次、取消与反馈。
- `use-subagent-task-thread.ts` 只组合资源和投影，不直接调用 API。
- `subagent-task-thread-view.tsx` 只消费窄视图模型，不解释请求或能力协议。
- `subagent-task-thread.tsx` 复用 controller 的 canonical sessionKey 绑定本地确认/输入弹窗，使用共享同步 reset 状态；返回旧任务不恢复已清空的确认或草稿。

任务标题复用任务名称投影和当前语言的 Agent/Subagent 通称，不能展示内部任务 ID。详情 Header 直接使用公共 `UiSeededAvatar size="xs"`（32px），不从列表视图导入头像并覆盖其尺寸。底部操作区使用实体 panel token、单条分隔与公共 metadata 排版，说明和按钮组可按可用宽度换行；只有当前命令设置 aria-busy，pending 期间保持原有防重规则。

runtime 的 task.agent_id 只是任务/接收者身份。文件工作区来自 task.host_agent_id、消息已有的宿主 Agent 或 Artifact 明确来源；传给 Thread 的空工作区必须保持为空，不回退 task ID，也不在预览回调覆盖文件已经解析的 owner。完整链路继续服从共享 Thread 与 Artifact 适配器的来源合同。

线程不伪装成普通 Composer；只在服务端 capability 允许时提供停止和“补充指令/继续任务”，停止必须二次确认。作用域切换后，旧查询或 mutation 均不得写回当前任务。

线程初始加载和空记录使用 `UiResourceState` 的 plain/sm，加载图标使用共享 `md` Spinner，停止、补充指令和继续任务使用 `sm`；视图不得自行维护颜色、旋转或 reduced-motion class。读取失败保留已有输出，缺少成功快照时不同时声称“暂无记录”；文本输出按 code 角色保留原始换行与字面文本。
