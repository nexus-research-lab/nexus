# Provider 模型动作

- `use-provider-model-actions.ts` 只装配模型状态、同步、添加、更新和测试动作。
- `use-provider-model-controls.ts` 只持有搜索、弹窗和编辑草稿状态。
- `use-provider-model-sync.ts` 负责保存 Provider 后同步远端模型目录。
- `use-provider-model-add.ts` 负责校验并添加手工模型。
- `use-provider-model-update.ts` 负责启停与参数更新，并统一默认模型禁用反馈。
- `use-provider-test-actions.ts` 负责 Provider 与模型连通性测试。
- `use-provider-persisted-model-command.ts` 统一“持久化配置、执行请求、刷新目标、发布反馈”的事务骨架。
- 每个命令 Hook 只声明自己消费的 `ProviderModelApi` 子集，不依赖完整 API 门面。
