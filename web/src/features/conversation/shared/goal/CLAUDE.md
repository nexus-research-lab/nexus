# Goal Panel

- `use-goal-resource.ts` 负责带会话键的 Goal + owner-scoped server-derived Execution binding 资源快照、刷新版本，以及命令事务的开始、解析、拒绝、只读对账和收尾阶段。绑定读取失败必须 fail closed，不得解析 Goal metadata 补猜。
- `goal-lifecycle-recovery.ts` 保存 exact Session/Goal 生命周期意图和保守对账纯模型；`goal-reliability-notice.tsx` 把资源、权限、mutation 与 Goal runtime 事实展示为 Problem / Impact / Recovery，不解释内部 ID。预算耗尽、续跑限制和一般 runtime 失败使用各自可执行恢复文案。
- 页面 Composer 通过独立 `set_goal` WebSocket 控制消息创建 Goal；它与文本 `/goal` 共用后端 host handler，不调用 REST create，也不把 objective 送入普通 chat/runtime。服务端 Goal 事件只触发当前 session 的资源刷新，资源继续以 owner-scoped `GET current + binding` 为权威快照。
- `use-goal-controller.ts` 只维护编辑草稿、确认弹窗和用户动作编排；可见状态由纯模型投影。清除只在 binding 为 `standalone|reserved` 时开放，`pending|confirmed|conflict` 或读状态缺失时禁用并给出原因；后端仍是最终 gate。
- 资源快照同时携带 `sessionKey` 与进程内 auth owner generation；会话或 owner 变化必须同步隐藏并清除旧 Goal、旧 mutation 锁和迟到回调。owner generation 只是客户端 scope fence，不是业务 ID、幂等键或持久日志。
- 刷新请求通过版本号拒绝过期响应；写命令全局互斥，并使在途读取失效。
- current Goal 主读取失败时保留同 owner/Session 的上次快照但锁住生命周期修改，直到只读刷新成功；只有 binding 辅助读取失败时可继续不依赖 binding 的修改，清除仍保持 fail closed。未知 update 期间草稿保留但不可修改或提交，用户仍可主动关闭；确认写入后不保留一份会诱导重复保存的旧草稿。
- 生命周期写入只有服务端明确返回 `not_applied` 才能直接解锁重试。`accepted`、transport failure 和没有 FailureCore 的旧响应都按结果未知处理，当前页面锁定 exact Session/Goal 意图并只用 owner-scoped current Goal 读取核对；读取未变化不能证明原请求不会稍后提交。权威状态已经达到目标时可以解除当前页面锁；原 Goal 不再是 current 时也只解除当前面板的锁，并明确保留“旧操作仍可能影响原 Goal 历史”的数据影响。其他并发版本变化、objective rewrite 和不一致结果必须继续 fail closed。锁不跨页面刷新持久化，前端不得声称具备持久防重。
- mutation 响应成功后先采用该响应的权威结果，再单独刷新 Goal/binding；后续读取失败只能报告“写入已完成、刷新失败”，不得降格成写入失败。401/403 或 Session 切换必须清除可见旧快照并拒绝旧异步结果，任何恢复都不得自动重发 lifecycle mutation、prompt 或其他副作用。
- 编辑表单使用单一草稿对象，并采用无装饰图标与副标题的 plain chrome；只有清除 Goal 需要确认弹窗，状态恢复由面板内显式动作直接执行。
- `goal-panel.tsx` 只组合状态条、编辑弹窗和单一确认弹窗，不直接调用 API。
- `goal-model.ts` 统一 Goal 生命周期、有意义的 server-derived WorkGraph binding 徽标与清除能力、实际 token 用量、预算表单、控制器可见性、动作规则与外部活动版本的纯投影；Goal 活跃但没有执行时显示“运行中”，真实生成期间以同一个主状态原位替换为“执行中”，禁止同时展示两个同层状态。状态条只展示一个实际用量数字，估算值以 `≈` 标记，complete 但尚未 finalized 时隐藏 token，不展示预算计量、进度条或用量 tooltip。
- `status=paused` 只投影为真实“已暂停”；active Goal 的自动续跑状态只消费服务端 `continuation_state`，`recovering` 仍是 active，只有 `suspended` 显示“自动续跑已停止”及“不是 Agent 主动暂停”的行内原因，并保留继续动作。前端不得根据 `empty_progress_count` 重建门槛。Plan/权限 hold 使用服务端 hold 自己的 label/detail，不与前两者合并。
- `goal-status-strip.tsx` 只渲染状态模型并把动作分发给控制器，不解释 Goal 运行规则。Goal 的 lifecycle/activity 共用一个主状态槽并复用 `UiBadge`，预算/耗时只是无壳 Typography 元信息，不得再生成 Goal 私有胶囊；`standalone|reserved` 不显示冗余 binding 徽标，但服务端状态仍负责清除授权；`pending|confirmed|conflict` 分别显示确认中、已关联和冲突，读取失败显示状态不可用并保持 fail closed。
- Goal 状态条属于 Composer 向上工作栈的第一层；桌面使用略窄于 Composer 的内容 lane、圆角浮层和 8px 层间距，移动端沿用紧凑 lane。长目标保持单行截断并保留完整 DOM 文本与悬停标题，不能把运行控制条铺满画布。
- 编辑提交和状态刷新统一使用共享 `md` Spinner；非加载状态保留原生命周期动作图标，Goal 视图不得维护旋转或 reduced-motion class。
