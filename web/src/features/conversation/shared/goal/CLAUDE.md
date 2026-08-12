# Goal Panel

- `use-goal-resource.ts` 负责带会话键的 Goal + owner-scoped server-derived Execution binding 资源快照、刷新版本，以及命令事务的开始、解析、拒绝和收尾阶段。绑定读取失败必须 fail closed，不得解析 Goal metadata 补猜。
- 页面 Composer 通过独立 `set_goal` WebSocket 控制消息创建 Goal；它与文本 `/goal` 共用后端 host handler，不调用 REST create，也不把 objective 送入普通 chat/runtime。服务端 Goal 事件只触发当前 session 的资源刷新，资源继续以 owner-scoped `GET current + binding` 为权威快照。
- `use-goal-controller.ts` 只维护编辑草稿、确认弹窗和用户动作编排；可见状态由纯模型投影。清除只在 binding 为 `standalone|reserved` 时开放，`pending|confirmed|conflict` 或读状态缺失时禁用并给出原因；后端仍是最终 gate。
- 资源快照携带 `sessionKey`，视图不得在会话切换期间展示旧 Goal。
- 刷新请求通过版本号拒绝过期响应；写命令全局互斥，并使在途读取失效。
- 编辑表单使用单一草稿对象；只有清除 Goal 需要确认弹窗，状态恢复由面板内显式动作直接执行。
- `goal-panel.tsx` 只组合状态条、编辑弹窗和单一确认弹窗，不直接调用 API。
- `goal-model.ts` 统一 Goal 生命周期、server-derived binding 徽标与清除能力、实际 token 用量、预算表单、控制器可见性、动作规则与外部活动版本的纯投影；状态条只展示一个实际用量数字，估算值以 `≈` 标记，complete 但尚未 finalized 时隐藏 token，不展示预算计量、进度条或用量 tooltip。
- `status=paused` 只投影为真实“已暂停”；active Goal 的 `empty_progress_count>0` 必须显示“自动续跑已停止”及“不是 Agent 主动暂停”的行内原因，并保留继续动作，不能再复用含糊的“待继续”。Plan/权限 hold 使用服务端 hold 自己的 label/detail，不与前两者合并。
- `goal-status-strip.tsx` 只渲染状态模型并把动作分发给控制器，不解释 Goal 运行规则。生命周期徽标始终是主状态；`standalone|reserved` 是内部绑定状态，不渲染次级徽标，只有 `pending|confirmed|conflict` 分别显示确认中、已关联和冲突，读取失败显示状态不可用并保持 fail closed。
- Goal 状态条属于 Composer 向上工作栈的第一层；桌面使用略窄于 Composer 的内容 lane、圆角浮层和 8px 层间距，移动端沿用紧凑 lane。长目标保持单行截断并保留完整 DOM 文本与悬停标题，不能把运行控制条铺满画布。
