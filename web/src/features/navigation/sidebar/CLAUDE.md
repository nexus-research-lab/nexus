# 应用宽侧栏

- `sidebar-wide-panel.tsx` 只组合折叠/展开视图和唯一引导中心弹层；手机一级目录强制展开并占满可用宽度，同时把系统操作收进左侧 Dock。
- `use-sidebar-wide-panel-controller.ts` 独占路由、认证、通知与 Sidebar Store 装配；退出入口只在网页端已进入密码认证会话时显示，单用户免认证、访问令牌与桌面运行时均不得暴露无效退出动作；手机目录中的能力 Tab 进入 `/capability`，桌面仍直接打开默认能力页。
- `sidebar-wide-panel-model.ts` 纯派生主 Tab 与标签。
- `use-sidebar-panel-resize.ts` 只管理拖拽边界，不读取 Store。
- `view/` 只处理折叠与展开布局；顶部品牌栏只承载 Launcher 字标与有效的退出/折叠控制，聊天、联系人和能力共同进入带短标签的一级导航 Dock；Nexus 主智能体不再拥有独立 Dock 动作或 Focus 侧栏，而是复用聊天目录行。桌面 56px 收起态顶部只保留展开按钮，不显示 `N` 或退出；手机全宽目录顶部保留品牌与有效退出入口，底部只放设置和引导。各状态必须复用 `desktop-rail` 的主题底面，让导航、目录和主画布通过相邻中性灰阶分区；只有与主画布相邻的桌面侧栏绘制置顶且不透明的全高外缘 hairline，手机全宽目录和独立设置侧栏不得误用。

路由到主 Tab 的映射保持单一来源；业务引导统一由 `features/onboarding/` 提供。
