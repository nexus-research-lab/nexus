# 能力页共享语法

- `capability-page-layout.tsx` 只拥有能力域的页面编排、筛选节奏、目录网格、分区标题和无品牌身份框；不得解释 Skill、Connector、Channel、Loop、WorkGraph 或定时任务状态。
- 页面标题、用途说明和主动作必须通过 `CapabilityPageLayout` 进入全站 `WorkspaceContentHeader`；业务页面不得复制 Header、用绝对定位模拟右侧动作，或在移动端重复页面身份。
- `actions` 在桌面进入内容 Header 右侧，在存在应用页头目标时通过 Portal 投影到移动端页头；两种布局必须渲染同一个动作节点和业务行为。
- 搜索与筛选只组合共享 Form/Menu 原语；目录只组合 `UiListRow` 或所属领域卡片，不在本文件保存请求和筛选状态。
- App chrome 文字必须选择共享 Typography role；图标框和目录表面只使用语义圆角与边框，不得恢复页面私有字号、行高、字距、圆角或阴影。
- 修改布局、移动动作投影或分区层级时，必须同步维护同目录 DOM 测试与 `frontend-foundation-contract` 静态门禁。
