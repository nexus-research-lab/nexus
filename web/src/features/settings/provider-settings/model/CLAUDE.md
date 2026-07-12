# Provider Settings Model

- `provider-preset-model.ts` 解释预设、格式与 Provider 类型的兼容关系，并按预设选择、身份和格式阶段构造草稿。
- `provider-config-model.ts` 负责草稿、校验以及创建、更新、启停请求载荷。
- `provider-model-model.ts` 负责模型过滤、排序、新增/更新载荷与默认模型保护判定。
- `provider-catalog-model.ts` 只处理 Provider 列表顺序和选择。
- `provider-settings-presentation.ts` 集中目录、格式、标题与能力标志等纯展示投影，不读取交互状态。
- 模型层保持纯函数，不发请求、不持有 React 状态。
