# Agent Options 编辑域

本目录负责 Agent 身份、模型与权限配置的编辑状态和保存事务。

## 职责边界

- `agent-options-draft.ts` 定义单一草稿对象、编辑作用域键和保存载荷纯投影。
- `use-agent-options-draft.ts` 只管理草稿字段与工具集合变更。
- `use-agent-provider-options.ts` 只管理 runtime 维度的 Provider 目录请求。
- `use-agent-profile-template.ts` 只在创建来源下加载服务端默认行为模板；控制器把成功结果写入当前草稿一次，用户随后清空或修改时不得被异步结果覆盖。
- `use-agent-name-validation.ts` 统一 debounce 与保存前名称校验，错误结果只在一处构造。
- `use-agent-save-feedback.ts` 只管理保存反馈及其生命周期。
- `use-agent-options-save-command.ts` 按准入、校验、持久化和结果归属执行保存事务，并用同步令牌拒绝重复提交。
- `use-agent-options-editor-controller.ts` 只组合上述状态并向视图提供内容与动作模型。

## 不变量

- 编辑作用域直接包含规范化来源对象；`edit` 来源必须携带具体 Agent ID，`create` 来源不得伪造空 ID。
- 保存和名称校验的异步结果必须同时匹配当前作用域与草稿版本。
- 同一作用域的保存命令必须串行；React 状态尚未提交时仍由同步令牌拒绝重复点击。
- 身份、Provider 与权限字段属于同一个草稿，不得拆成互相同步的镜像 state。
- Provider 加载错误、名称校验错误和保存反馈各自归属独立状态，不得互相覆盖。
- 创建草稿中的 `profileTemplate` 与数据库摘要 `description` 是两个字段；前者只进入创建协议的 `profile_template` 并由后端写入 AGENTS.md。
