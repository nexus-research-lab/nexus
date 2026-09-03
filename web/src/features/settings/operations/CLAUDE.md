# operations/

L3 | 父级: web/src/features/settings

## 职责

- `operations-access.ts` 定义运营分区的角色准入规则。
- `operations-panel.tsx` 通过单一 Tab 定义表装配部署成员、订阅、公共 Provider 与项目权限管理视图；固定页面标题位于第一行，纯文字 Tab 位于第二行，子视图不得重复页面级 Header 与内容轴。
- `control-members-panel.tsx` 消费 Control 成员 API，负责创建、角色与 active/revoked 状态；未知写结果先刷新清单，不自动重放。
- `subscription-admin/` 负责订阅账号、套餐草稿与写事务；Control 提供套餐和成员 entitlement，Nexus 只提供本地 token 用量。
- `project-admin/` 负责共享项目创建、成员 ACL 展示与管理员授权事务；角色权限仍由服务端判定。

运营是跨权威的设置管理分区，不拥有独立页面状态：成员与订阅属于 Control，公共 Provider 与项目 ACL 属于 Nexus 运行资源；旧 `/operations` 页面只负责权限校验和重定向。
