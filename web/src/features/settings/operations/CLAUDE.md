# operations/

L3 | 父级: web/src/features/settings

## 职责

- `operations-access.ts` 定义运营分区的角色准入规则。
- `operations-panel.tsx` 通过单一 Tab 定义表装配部署成员、订阅、公共 Provider 与项目权限管理视图；固定页面标题位于第一行，纯文字 Tab 位于第二行，子视图不得重复页面级 Header 与内容轴。
- `control-members-panel.tsx` 消费 Control 成员 API，负责创建、角色与 active/revoked 状态；未知写结果先刷新清单，不自动重放。
- `subscription-admin/` 负责订阅账号、套餐草稿与写事务；Control 提供套餐和成员 entitlement，Nexus 只提供本地 token 用量。
- `project-admin/` 负责共享项目创建、成员 ACL 展示与管理员授权事务；角色权限仍由服务端判定。

运营是跨权威的设置管理分区，不拥有独立页面状态：成员与订阅属于 Control，公共 Provider 与项目 ACL 属于 Nexus 运行资源；旧 `/operations` 页面只负责权限校验和重定向。
所有子视图复用 Settings Card、Control Label、语义 Typography、Badge 与 Resource State；不得在页面内重组字号/字重/行高、状态胶囊、加载空态或任意圆角。
成员刷新、项目命令和订阅 mutation 使用共享 `sm` Spinner；静止状态保留动作图标，Operations 视图不得拥有旋转、颜色或 reduced-motion class。
旧 `/operations` 路由的认证准入等待使用共享 `xl` primary Spinner；页面不得用边框 div 自制加载动画。
