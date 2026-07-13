# Provider Settings Actions

- `use-provider-command.ts` 是唯一异步互斥入口，进行中动作使用判别联合表达。
- `config/` 按字段联动、持久化和删除拆分 Provider 配置动作，并由薄装配 Hook 向控制器提供统一入口。
- `model/` 按交互状态、同步、添加、更新和测试拆分模型动作，并统一默认模型保护规则。
- 组合 hook 不重新实现请求流程，动作结束后由所属命令统一释放状态。
