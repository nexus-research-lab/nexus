# Provider Settings Model

- `provider-preset-model.ts` 解释预设、格式与 Provider 类型的兼容关系。
- `provider-config-model.ts` 负责草稿、校验和配置请求载荷。
- `provider-model-model.ts` 负责模型过滤、排序、参数与更新载荷。
- `provider-catalog-model.ts` 只处理 Provider 列表顺序和选择。
- `provider-settings-presentation.ts` 保存稳定的展示映射，不读取交互状态。
- 模型层保持纯函数，不发请求、不持有 React 状态。
