# 消息项显示状态

- `message-item-display-model.ts` 只从最小投影切片推导 Assistant 各区域可见性和动作能力。
- `use-process-expansion-lifecycle.ts` 只管理待确认、问题超时和直播模式切换引起的自动展开状态。
- 纯模型不得依赖投影 Hook 的完整返回类型，新增投影字段不应扩散到显示状态机。
- 过程可见性只由支持该布局的真实过程内容决定；未匹配权限由 Assistant 独立内容段展示，不得借空过程区承载。
