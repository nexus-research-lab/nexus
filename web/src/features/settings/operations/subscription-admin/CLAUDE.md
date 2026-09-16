# subscription-admin/

L4 | 父级: web/src/features/settings/operations

## 职责

- `subscription-admin-panel.tsx`: 订阅运营入口与视图装配
- `use-subscription-admin.ts`: 服务端快照、草稿修改和 mutation 编排
- `subscription-admin-model.ts`: 草稿、响应投影、校验与格式化纯函数
- `subscription-transaction.ts`: 同步互斥的加载与 mutation 事务
- `subscription-account-view.tsx`: 账号套餐分配
- `subscription-plan-view.tsx`: 套餐创建、编辑与保存
- 加载、空态、按钮、表单、Badge 和文本角色直接复用 `shared/ui` 与 Settings 域 Pattern，不保留 Subscription 私有视觉原语
- 账号刷新/保存和套餐创建/保存统一使用共享 `sm` Spinner；业务视图不维护独立动效配方

页面快照由 Control 套餐/成员 entitlement 与 Nexus 本地 token 用量组合；写请求只进入 Control，Nexus 不接受套餐或成员额度 mutation。加载与 mutation 在 React 状态
更新前竞争同一把同步锁，避免连续操作并发执行以及较晚返回的旧响应覆盖新操作。
读取失败保留最后成功的 overview 与草稿；mutation 必须按 `not_applied / accepted / committed / unknown` 投影影响和下一步，后三者在重新读取 overview 前禁止重复创建套餐、保存套餐或改写用户订阅。
禁写状态是独立于可见 feedback 的 mutation 事实；dismiss 、读取失败或其他提示替换都不得解锁。只有成功读取权威 overview 或服务端明确 `not_applied/committed` 才能收口。

不展示顶部账号数、套餐数及本月用量统计，账号行突出身份、用量/占比、套餐和当前有效额度；公共周期仅显示一次。会话/消息数与部署角色不重复出现在套餐分配视图。套餐目录直接按行编辑名称、状态、额度、排序与备注，行尾保存；新增套餐通过共享弹窗填写，关闭不丢失草稿，成功后重置草稿并关闭；新草稿使用共享 UUID 生成随机 Key，重试保留同一 Key，界面不显示 Key，修改显示名称保留原 Key。字段与保存动作底部对齐，目录操作使用 sm，展开表单使用 md，表单随工作面宽度分列。

账号页不显示常驻刷新，仅在未核对写入阻止继续管理时显示恢复入口：loading / mutationPending 时禁用，但 mutationsBlocked 不得禁用刷新；写控件继续同时遵循三种状态。

账号目录按身份、用量、套餐配置分列，保存为次要动作。套餐目录使用连续编辑行和一次公共表头，宽容器一行完成编辑与保存，窄容器自动换行并显示字段标签；新建入口按内容宽度显示，点击打开弹窗。

订阅账号目录在宽容器仅显示一次公共表头，行内不重复套餐与有效额度标签；窄容器隐藏表头并恢复行内标签。表头与行使用相同列宽，保存动作保留独立列。
