# 定时任务列表

- `scheduled-task-list.tsx` 只处理列表级加载、错误、空态和排序后的装配。
- `scheduled-task-list-item.tsx` 负责单项展示与动作菜单，不发起 API 请求。
- `scheduled-task-list-model.ts` 保存纯展示投影和排序规则。
- 列表命令全部为必选接口；该领域只有一个真实消费者，不保留无意义的可选兼容面。
