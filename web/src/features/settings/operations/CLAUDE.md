# operations/

L3 | 父级: web/src/features/settings

## 职责

- `operations-access.ts` 定义运营分区的角色准入规则。
- `operations-panel.tsx` 根据 URL 分区装配成员、订阅、套餐、公共 Provider 和项目页面；导航定义归设置侧栏，正文只保留当前子页标题，不拥有页签状态。
- `control-members-panel.tsx` 消费 Control 成员 API，负责创建、角色与 active/revoked 状态；未知写结果先刷新清单，不自动重放。
- `subscription-admin/` 负责订阅账号、套餐草稿与写事务；Control 提供套餐和成员 entitlement，Nexus 只提供本地 token 用量。
- `project-admin/` 负责共享项目创建、成员 ACL 展示与管理员授权事务；角色权限仍由服务端判定。
- `operations-views.test.tsx` 验证套餐展开后的草稿与保存禁用行为；`web/browser-tests/operations.spec.ts` 使用隔离数据检查五个子页的响应式布局和控件边界。

运营是跨权威的设置管理分区，不拥有独立页面状态：成员与订阅属于 Control，公共 Provider 与项目 ACL 属于 Nexus 运行资源；旧 `/operations` 页面只负责权限校验和重定向。
所有子视图复用 Settings Card、Control Label、语义 Typography、Badge 与 Resource State；不得在页面内重组字号/字重/行高、状态胶囊、加载空态或任意圆角。
成员刷新、项目命令和订阅 mutation 使用共享 `sm` Spinner；静止状态保留动作图标，Operations 视图不得拥有旋转、颜色或 reduced-motion class。
旧 `/operations` 路由的认证准入等待使用共享 `xl` primary Spinner；页面不得用边框 div 自制加载动画。

运营子页共用壳层内容宽度，不单独居中限宽。成员与项目以目录和状态为主，新建表单使用公共 Disclosure 按需展开；成员名、项目 ID 和路径允许完整换行。表单依据自身容器宽度分列，普通输入、选择和提交为 md，目录内动作和角色选择为 sm。装饰图标与项目权限版本不占用主内容层级；实际权限限制与写结果反馈保持可见。
部署角色的新建字段和成员列表统一使用 UiSelectMenu；创建角色选项与列表禁用条件继续遵循当前身份权限。
