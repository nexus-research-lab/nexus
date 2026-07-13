# operations/

L3 | 父级: web/src/features/settings

## 职责

- `operations-access.ts` 定义运营分区的角色准入规则。
- `operations-panel.tsx` 通过单一 Tab 定义表装配订阅与公共 Provider 管理视图。
- `subscription-admin/` 负责订阅账号、套餐草稿与写事务。

运营是设置管理分区，不拥有独立页面状态；旧 `/operations` 页面只负责权限校验和重定向。
