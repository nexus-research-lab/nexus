# Agent Options 编辑域

本目录负责 Agent 身份、模型与权限配置的编辑状态和保存事务。

## 职责边界

- `agent-options-draft.ts` 定义单一草稿对象、来源重置键、稳定命令作用域键和保存载荷纯投影。
- `use-agent-options-draft.ts` 只管理草稿字段与工具集合变更。
- `use-agent-provider-options.ts` 只管理 runtime 维度的 Provider 目录请求。
- `use-agent-connectors.ts` 只在工具页可见时加载可挂载的 Connector 目录。
- `use-agent-profile-template.ts` 只在创建来源下加载服务端默认行为模板；控制器把成功结果写入当前草稿一次，用户随后清空或修改时不得被异步结果覆盖。
- `agent-name-validation.ts` 与 `use-agent-name-validation.ts` 统一本地格式预检、debounce 与保存前校验；名称允许重复，不请求“可用性”接口，服务端提交仍是最终校验边界。
- `use-agent-save-feedback.ts` 只管理保存反馈及其生命周期。
- `agent-options-save-transaction.ts` 定义保存结果的命令作用域、用户草稿版本与来源版本归属规则。
- `use-agent-options-save-command.ts` 按准入、校验、持久化和结果归属执行保存事务，并用同步令牌拒绝重复提交。
- `use-agent-options-auto-save.ts` 只负责既有 Agent 草稿的延迟调度；同一失败版本不自动重试，用户产生新版本后才重新提交。
- `use-agent-options-editor-controller.ts` 只组合上述状态并向视图提供内容与动作模型。

## 不变量

- 来源重置键直接包含规范化来源对象；`edit` 来源必须携带具体 Agent ID，`create` 来源不得伪造空 ID。
- 命令作用域只由可见性、编辑模式和具体 Agent ID 构成；服务端回写内容不得伪装成编辑器切换。
- 保存前的名称校验结果必须匹配来源与用户草稿版本；持久化成功后允许同一 Agent 的来源被服务端回写，但不得覆盖更新的用户草稿。
- 同一命令作用域的保存命令必须串行；React 状态尚未提交时仍由同步令牌拒绝重复点击。
- 服务端来源回写只覆盖干净草稿；自动保存期间产生的后续用户修改必须保留，并在当前请求结束后作为新版本继续保存。
- 身份、Provider 与权限字段属于同一个草稿，不得拆成互相同步的镜像 state。
- Provider 加载错误、名称校验错误和保存反馈各自归属独立状态，不得互相覆盖。
- 创建草稿中的 `profileTemplate` 与数据库摘要 `description` 是两个字段；前者只进入创建协议的 `profile_template` 并由后端写入 AGENTS.md。
