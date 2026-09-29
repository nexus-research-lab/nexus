# Provider 模型动作

- `use-provider-model-actions.ts` 只装配模型状态、同步、添加、更新和测试动作。
- `use-provider-model-controls.ts` 只持有搜索、弹窗和编辑草稿状态。
- `use-provider-model-sync.ts` 负责保存 Provider 后快速同步远端模型目录，不启动能力探测。
- `use-provider-model-add.ts` 负责校验并添加手工模型。
- `use-provider-model-update.ts` 负责启停与参数更新，并统一默认模型禁用反馈。
- `provider-capability-batch.ts` 负责最多三个模型同时验证的调度、完成进度、停止后续请求和失败收口，不重发网络请求。
- `use-provider-test-actions.ts` 负责七类能力验证，全部模型使用有界并发调度器并支持进度与停止；未确认与明确不支持独立反馈。
- `use-provider-persisted-model-command.ts` 统一“持久化配置、执行请求、刷新目标、发布反馈”的事务骨架。
- 每个命令 Hook 只声明自己消费的 `ProviderModelApi` 子集，不依赖完整 API 门面。

- 删除模型成功但目录刷新失败时，复用已提交/刷新失败反馈，不能以成功 toast 覆盖工作区读失败；不自动重试删除。测试反馈只保留实际消费的字段。
