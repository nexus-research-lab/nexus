# settings/

L2 | 父级: web/src/features

## 职责

- 根目录定义设置分区、URL 导航和页面装配。
- `general/`、`personal/` 与 `provider-settings/` 各自拥有设置资源和交互。
- `operations/` 是设置管理分区，负责角色准入、订阅运营与公共 Provider 管理装配。
- `shared/` 只保存多个设置分区共用的展示原语。

设置域内部可以组合兄弟分区；不得再建立独立顶层 Operations Feature 反向依赖设置域。
