# 应用宽侧栏

- `sidebar-wide-panel.tsx` 只组合折叠/展开视图和唯一引导中心弹层；手机一级目录强制展开并占满可用宽度，同时把系统操作收进左侧 Dock。
- `use-sidebar-wide-panel-controller.ts` 独占路由、认证、通知、目录摘要与 Sidebar/Room Navigation Store 装配；固定会话点击进入 exact Conversation，拖放只重排持久偏好，X 只取消固定；退出入口只在网页端已进入密码认证会话时显示，单用户免认证、访问令牌与桌面运行时均不得暴露无效退出动作；手机目录中的能力 Tab 进入 `/capability`，桌面仍直接打开默认能力页。
- `sidebar-wide-panel-model.ts` 纯派生主 Tab、固定会话标题/路由与标签。
- `use-sidebar-panel-resize.ts` 只管理拖拽边界，不读取 Store。
- `view/sidebar-panel.tsx` 用单一常驻壳层处理展开与收起；顶部品牌栏只承载 Launcher 字标与折叠控制，聊天、联系人、能力和固定会话共同对齐 64px Dock 的水平中心轴，主 Tab 与固定会话必须复用同一 32px 图标框表达选中和 hover；有固定会话时，`sidebar-pinned-conversations.tsx` 仅在展开态于能力下方显示短分割线，继续同一 Dock 几何并支持按落点拖放排序，X 位于右上外沿且只提交取消固定。Nexus 主智能体不再拥有独立 Dock 动作或 Focus 侧栏，而是复用聊天目录行。桌面收起宽度固定为 68px（4px 左侧舞台留边 + 64px Dock），展开控制、主 Tab 与固定项的图标中心轴不得随收起状态变化；收起态使用单一导航底面、隐藏底部操作并移除内部全部分隔，目录只做裁切与淡出。手机全宽目录顶部只保留品牌。设置、引导、桌面更新提示与有效退出仅在展开态进入底部操作区；更新提示只在桌面桥接确认有新版本时出现并固定在右下，退出保持为最右侧系统动作。各状态必须复用 `desktop-rail` 的主题底面，让导航、目录和主画布通过相邻中性灰阶分区；只有与主画布相邻的桌面侧栏绘制置顶且不透明的全高外缘 hairline，手机全宽目录和独立设置侧栏不得误用。

路由到主 Tab 的映射保持单一来源；业务引导统一由 `features/onboarding/` 提供。
