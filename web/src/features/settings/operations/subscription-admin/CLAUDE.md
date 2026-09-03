# subscription-admin/

L4 | 父级: web/src/features/settings/operations

## 职责

- `subscription-admin-panel.tsx`: 订阅运营入口与视图装配
- `use-subscription-admin.ts`: 服务端快照、草稿修改和 mutation 编排
- `subscription-admin-model.ts`: 草稿、响应投影、校验与格式化纯函数
- `subscription-transaction.ts`: 同步互斥的加载与 mutation 事务
- `subscription-account-view.tsx`: 用户订阅概览和账号套餐分配
- `subscription-plan-view.tsx`: 套餐创建、编辑与保存
- 加载、空态、按钮、表单、Badge 和文本角色直接复用 `shared/ui` 与 Settings 域 Pattern，不保留 Subscription 私有视觉原语
- 账号刷新/保存和套餐创建/保存统一使用共享 `sm` Spinner；业务视图不维护独立动效配方

服务端 `overview` 与其草稿映射必须通过一次快照提交保持原子一致；加载与 mutation 在 React 状态
更新前竞争同一把同步锁，避免连续操作并发执行以及较晚返回的旧响应覆盖新操作。
读取失败保留最后成功的 overview 与草稿；mutation 必须按 `not_applied / accepted / committed / unknown` 投影影响和下一步，后三者在重新读取 overview 前禁止重复创建套餐、保存套餐或改写用户订阅。
禁写状态是独立于可见 feedback 的 mutation 事实；dismiss 、读取失败或其他提示替换都不得解锁。只有成功读取权威 overview 或服务端明确 `not_applied/committed` 才能收口。
