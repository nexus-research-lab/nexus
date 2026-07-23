# 应用宽侧栏

- `sidebar-wide-panel.tsx` 只组合折叠/展开视图和唯一引导中心弹层；手机一级目录强制展开并占满可用宽度，同时把系统操作收进左侧 Dock。
- `use-sidebar-wide-panel-controller.ts` 独占路由、认证、通知与 Sidebar Store 装配；手机目录中的能力 Tab 进入 `/capability`，桌面仍直接打开默认能力页。
- `sidebar-wide-panel-model.ts` 纯派生主 Tab、标签和 Nexus 激活状态。
- `use-sidebar-panel-resize.ts` 只管理拖拽边界，不读取 Store。
- `view/` 只处理折叠与展开布局，共用主 Tab、Nexus 入口和系统操作；桌面展开态与手机一级目录都把聊天、联系人、能力放在带短标签的左侧一级导航 Dock，桌面收起态退化为纯图标 Rail。手机目录的设置、引导与退出也进入 Dock 底部，不得重新铺成整行 Footer。各状态必须复用远端 `desktop-rail` 的透明底面，让页面纹理连续透出，只以低对比边界划分导航区域。

路由到主 Tab 的映射保持单一来源；业务引导统一由 `features/onboarding/` 提供。
