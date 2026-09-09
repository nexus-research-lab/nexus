# settings/

L2 | 父级: web/src/features

## 职责

- 根目录定义设置分区、URL 导航和页面装配。
- `general/`、`runtime/`、`browser/`、`personal/` 与 `provider-settings/` 各自拥有设置资源和交互；`general/` 管理当前用户所有 Agent 共享的 Echo 开关，`browser/` 只在桌面端展示扩展安装、连接状态和完整 CDP 权限。
- `operations/` 是设置管理分区，负责角色准入、订阅运营与公共 Provider 管理装配。
- `shared/` 只保存多个设置分区共用的展示原语。
- `shared/settings-panel-ui.tsx` 的 `SettingsToggleRow` 统一常规行为和工具发现的开关行、具名控件及实例说明关联；各分区仍传入自身禁用条件与回调。Browser 风险卡和模型表单保持领域组合。
- 各设置分区的正文必须复用 `shared/ui/layout/workspace-content-layout.ts` 的铺满管理内容面和共享水平留白，不得各自维护页面宽度与水平 padding。
- 设置分区在桌面使用 `WorkspaceContentHeader` 组合唯一标题、一句用途说明与页面动作；手机由应用栏显示当前 URL 分区名称，正文 Header 隐藏，不能同时出现笼统“设置”和第二个同义标题。行级说明可见，用于解释选项影响。
- 桌面独立设置窗口的宽屏导航使用文字 panel；窄于 `sm` 时必须收为同一 `SettingsSidebarNavigation` 的图标 rail，正文获得剩余工作面，不得继续保留固定宽度侧栏挤压内容。
- 运营分区保持固定“运营管理”标题，纯文字切换入口位于下一行；订阅、套餐、公共 Provider 与项目权限子视图只提供内容，不重复 Header。
- App chrome 文本必须选择 `shared/ui/typography` 的语义角色，状态、空态、按钮、表单与 Badge 必须复用对应共享所有者；页面不得直接组合字号、字重、行高、字距或任意圆角。
- 设置页主内容加载使用共享 Spinner 的 `lg`，按钮、选择器与局部命令使用 `xs/sm/md`；各分区不得自行拼接旋转、颜色或 reduced-motion class。

设置域内部可以组合兄弟分区；不得再建立独立顶层 Operations Feature 反向依赖设置域。

设置侧栏搜索复用 UiSearchInput 和公共字段匹配器，按本地化入口/分组名称及具体设置项名称、说明筛选；保留权限过滤，空分组隐藏，图标 rail 不受搜索草稿影响。`settings-sidebar-navigation.test.tsx` 覆盖筛选、清空和入口选择。

`settings-search-items.ts` 登记各设置模块可搜索的现有文案键，搜索结果在模块下方列出匹配内容；点击通过 target 文案键进入所属模块并定位设置项，始终沿用平台与管理员入口过滤，不索引用户资料值或 API 密钥。

`use-settings-search-target.ts` 在内容容器内定位搜索目标，等待异步加载并在再次点击相同结果时重新滚动；只处理当前模块登记的文案键，不触发字段操作。对应测试验证真实路由和延迟内容定位。
