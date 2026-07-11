# Provider 配置动作

- `use-provider-config-actions.ts` 只装配配置字段、持久化和删除动作，不承载具体业务规则。
- `use-provider-config-fields.ts` 负责 Provider 类型与 API 格式的联动；默认格式使用映射表表达，避免在事件处理器中堆叠类型特判。
- `use-provider-persistence.ts` 负责配置校验、创建或更新、失焦自动保存及启停回滚，并向模型和测试动作暴露窄的 `PersistProvider` 命令契约。
- `use-provider-delete.ts` 负责删除确认、占用提示和删除命令；弹窗状态使用单一判别联合，禁止并行布尔状态。
