# settings/

L2 | 父级: web/src/features

## 职责

- 根目录定义设置分区、URL 导航和页面装配。
- `general/`、`runtime/`、`browser/`、`personal/` 与 `provider-settings/` 各自拥有设置资源和交互；`general/` 管理当前用户所有 Agent 共享的 Echo 开关，`browser/` 只在桌面端展示扩展安装、连接状态和完整 CDP 权限。
- `operations/` 是设置管理分区，负责角色准入、订阅运营与公共 Provider 管理装配。
- `shared/` 只保存多个设置分区共用的展示原语。
- 各设置分区的正文必须复用 `shared/ui/layout/workspace-content-layout.ts` 的铺满管理内容面和共享水平留白，不得各自维护页面宽度与水平 padding。
- 设置分区在桌面使用 `WorkspaceContentHeader` 组合唯一标题、一句用途说明与页面动作；手机由应用栏显示当前 URL 分区名称，正文 Header 隐藏，不能同时出现笼统“设置”和第二个同义标题。行级说明可见，用于解释选项影响。
- 桌面独立设置窗口的宽屏导航使用文字 panel；窄于 `sm` 时必须收为同一 `SettingsSidebarNavigation` 的图标 rail，正文获得剩余工作面，不得继续保留固定宽度侧栏挤压内容。
- 运营分区保持固定“运营管理”标题，纯文字切换入口位于下一行；订阅、套餐、公共 Provider 与项目权限子视图只提供内容，不重复 Header。
- App chrome 文本必须选择 `shared/ui/typography` 的语义角色，状态、空态、按钮、表单与 Badge 必须复用对应共享所有者；页面不得直接组合字号、字重、行高、字距或任意圆角。

设置域内部可以组合兄弟分区；不得再建立独立顶层 Operations Feature 反向依赖设置域。
