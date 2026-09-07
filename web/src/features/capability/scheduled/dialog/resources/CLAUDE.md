# Task Dialog Resources

- 所有依赖请求共用 `use-dialog-resource.ts` 的请求号与过期响应拒绝逻辑。
- `task-dialog-resource-model.ts` 统一生成请求键、选项和当前会话资源投影；Hook 只执行请求并组合结果。
- Agent 选项复用公共选择文字适配；Room 成员候选使用完整 Agent 目录作为显示上下文，目录不能扩大 exact Session/成员/暂停过滤得到的候选。执行者仍取当前 conversation 的可用成员，投递署名仍从接收 conversation 的真实 Session 去重生成；两者独立保留默认房主资格。名称/语言更新只更新显示，选项 value、Session key 与表单绑定不变。
- `task-dialog-selection-labels.ts` 将 Room/Session 标签交给中立序号算法；Room 在资格筛选前用完整真实群聊目录生成标签，执行与投递保持一致，DM-backed Room 不参与编号。Session 只显示标题与来源徽标，同名时按精确 key 编号；上方字段已表达父身份，不重复名字或维护 Agent/Room 名称索引。文字规则只在根 `design.md` 维护。
- DM 执行和投递的单项显示由资源模型内部 `buildAgentSessionOption` 共用；时间模型只负责时间，不提供 Session 标签 helper。Room 的成员 Session 仍按真实 conversation 折叠，不因显示同名而合并身份。
- 历史脚本任务不加载会话；Agent 任务仅在执行或投递确实需要时加载会话。Agent 候选只接收结构化 DM/active-paired IM Session（Room-backed DM 即使带 `room_id` 仍合法）；Room 候选只接收真实 group Room，把成员 Session 折叠为共享 conversation，再从该 conversation 的实际 Session 索引生成成员 Agent 候选。
- 资源层返回具体选项、`loading/error` 和同一读取的手动重试动作，不持有表单选择。同 key 刷新失败保留上次成功选项，跨 key 不复用旧身份快照。
- 表单只消费资源的安全状态和重试动作，就近回答 Problem / Impact / Recovery；原始异常和资源项不得越过资源层。
