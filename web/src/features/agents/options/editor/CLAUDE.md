# Agent Options 编辑域

本目录负责 Agent 身份、模型与权限配置的编辑状态和保存事务。

## 职责边界

- `agent-options-draft.ts` 定义单一草稿对象、编辑作用域键和保存载荷纯投影。
- `use-agent-options-draft.ts` 只管理草稿字段与工具集合变更。
- `use-agent-provider-options.ts` 只管理 runtime 维度的 Provider 目录请求。
- `use-agent-name-validation.ts` 统一 debounce 与保存前名称校验，错误结果只在一处构造。
- `use-agent-save-feedback.ts` 只管理保存反馈及其生命周期。
- `use-agent-options-editor-controller.ts` 组合上述状态并向视图提供窄 Props。

## 不变量

- 编辑作用域必须包含 Agent 身份、模式和初始配置；相同字段值不等于同一个 Agent。
- 保存和名称校验的异步结果必须同时匹配当前作用域与草稿版本。
- 身份、Provider 与权限字段属于同一个草稿，不得拆成互相同步的镜像 state。
- Provider 加载错误、名称校验错误和保存反馈各自归属独立状态，不得互相覆盖。
