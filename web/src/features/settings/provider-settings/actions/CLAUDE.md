# Provider Settings Actions

- `use-provider-command.ts` 是唯一异步互斥入口，进行中动作使用判别联合表达。
- `config/` 按字段联动、持久化和删除拆分 Provider 配置动作，并由薄装配 Hook 向控制器提供统一入口。
- `use-provider-model-controls.ts` 只持有搜索、弹窗与编辑草稿状态。
- `use-provider-model-mutations.ts` 负责模型同步、添加、启停和参数更新。
- `use-provider-test-actions.ts` 只负责 Provider 与模型连通性测试。
- 组合 hook 不重新实现请求流程，动作结束后由所属命令统一释放状态。
