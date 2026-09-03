# 能力页共享语法

- `capability-page-layout.tsx` 只拥有能力域的页面编排、筛选节奏、目录网格、分区标题、详情页内容轴/二级导航、对象身份区、阅读列/配置侧栏和无品牌身份框；不得解释 Skill、Connector、Channel、Loop、WorkGraph 或定时任务状态。
- 页面标题、用途说明和主动作必须通过 `CapabilityPageLayout` 进入全站 `WorkspaceContentHeader`；业务页面不得复制 Header、用绝对定位模拟右侧动作，或在移动端重复页面身份。
- `actions` 在桌面进入内容 Header 右侧，在存在应用页头目标时通过 Portal 投影到移动端页头；两种布局必须渲染同一个动作节点和业务行为。
- 搜索与筛选只组合共享 Form/Menu 原语；目录只组合 `UiListRow` 或所属领域卡片，不在本文件保存请求和筛选状态。
- App chrome 文字必须选择共享 Typography role；图标框和目录表面只使用语义圆角与边框，不得恢复页面私有字号、行高、字距、圆角或阴影。
- 具有长正文和独立配置集合的能力详情统一使用 `CapabilityDetailSplitLayout`：宽工作面保持受控正文列与配置侧栏，窄窗口按“配置在前、长正文在后”收为单列；分区标题、说明和计数使用 `CapabilityDetailSectionHeader`，业务页不得复制断点与列宽。
- Skill、Connector、自定义 MCP、Loop 与 WorkGraph 的二级页必须由 `CapabilityDetailPage` 持有内容轴、导航后的统一正文起点，并由内部唯一的 `CapabilityDetailHeader` 组合全站 `UiBreadcrumb` 渲染“返回目录 / 当前对象”；业务页不得直接引用 `WorkspaceContentDetailHeader`、复制箭头、正文顶距和间距，目录态 Header/搜索也不得残留在详情路由上。
- 上述详情页在导航下方的对象图标、标题、标题元数据、说明和动作必须交给 `CapabilityDetailIdentity`；业务页只投影领域内容和动作资格，不得手写 `objectTitle`、响应式动作容器，或把目录态 `WorkspaceContentHeader` 当成对象 Header。
- Connector 与 Channel 的品牌身份都通过 `CapabilityBrandIcon` 渲染：领域只选择准确 SVG 和名称，容器、尺寸、单色主题前景及回退字形由共享组件持有；不得在卡片中恢复平台色背景或通用占位图标。
- 修改布局、移动动作投影或分区层级时，必须同步维护同目录 DOM 测试与 `frontend-foundation-contract` 静态门禁。
