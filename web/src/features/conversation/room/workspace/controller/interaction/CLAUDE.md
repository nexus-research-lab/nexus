# Workspace Interaction

Room Workspace 的本地交互与命令结果协调边界。

## 职责

- `workspace-interaction-model.ts`：Prompt 与菜单位置的纯状态模型。
- `use-workspace-navigation.ts`：当前文件、目录焦点和路径迁移结果。
- `use-workspace-interaction-state.ts`：菜单、Prompt、删除目标和上传目标状态。
- `use-workspace-entry-transactions.ts`：把上传、创建、重命名和删除结果提交给导航状态。

## 不变量

- API 命令仍由上层 `use-workspace-commands.ts` 独占，本目录只消费窄命令接口。
- 文件路径变化只通过导航控制器提交，不允许事务各自改写活动路径。
- Agent 切换必须重置全部本地交互状态。
