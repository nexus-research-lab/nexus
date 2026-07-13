# subscription-admin/

L4 | 父级: web/src/features/settings/operations

## 职责

- `subscription-admin-panel.tsx`: 订阅运营入口与视图装配
- `use-subscription-admin.ts`: 服务端快照、草稿修改和 mutation 编排
- `subscription-admin-model.ts`: 草稿、响应投影、校验与格式化纯函数
- `subscription-transaction.ts`: 同步互斥的加载与 mutation 事务
- `subscription-account-view.tsx`: 用户订阅概览和账号套餐分配
- `subscription-plan-view.tsx`: 套餐创建、编辑与保存
- `subscription-admin-ui.tsx`: 两个视图共用的基础控件状态

服务端 `overview` 与其草稿映射必须通过一次快照提交保持原子一致；加载与 mutation 在 React 状态
更新前竞争同一把同步锁，避免连续操作并发执行以及较晚返回的旧响应覆盖新操作。
