# browser/

L4 | 父级: web/src/features/settings

## 职责

- `browser-settings-section.tsx`: 浏览器扩展连接、安装恢复与 CDP 偏好的唯一设置视图。

扩展状态轮询只更新读取快照，不自动重放安装或偏好 mutation；安装动作由桌面 bridge 明确触发，CDP 偏好继续服从 Preferences 的版本与恢复合同。
页面身份复用 `WorkspaceContentHeader`，内容分组复用 Settings Card，连接状态复用 `UiBadge`，加载/失败/不兼容状态复用 `UiResourceState`。
标题、状态、说明、步骤与风险文案只选择 App Typography 语义角色；本目录不得自行拼接字号、行高、字重、tracking、任意圆角或状态徽标。
