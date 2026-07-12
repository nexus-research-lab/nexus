# Provider 配置动作

- `use-provider-config-actions.ts` 只装配配置字段、持久化和删除动作，不承载具体业务规则。
- `provider-config-field-model.ts` 用纯状态迁移表达 Provider 类型与 API 格式联动，并以判别结果拒绝不支持的格式。
- `use-provider-config-fields.ts` 只把显示名与字段事件映射到草稿补丁和反馈，不解释格式兼容规则。
- `provider-persistence-plan.ts` 用有序停止规则表达空态、只读、校验失败和无变更，不在 Hook 内堆叠提前返回。
- `use-provider-persistence.ts` 只编排创建或更新事务、失焦自动保存及启停回滚，并向模型和测试动作暴露窄的 `PersistProvider` 命令契约。
- `use-provider-delete.ts` 负责删除确认、占用提示和删除命令；弹窗状态使用单一判别联合，禁止并行布尔状态。
