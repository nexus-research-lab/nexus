# 通用布局原语

- 本目录只保留可跨页面复用的加载、Workspace 布局模型和面板拖拽入口。
- `app-loading-screen.tsx` 只用于启动、认证和首屏目录初始化等品牌级等待，使用本地 animated WebP 展示启动猫咪，并通过 `picture` 为减弱动效偏好切换静态帧；普通路由、Workspace、资源和按钮等待统一消费 `display/spinner-styles.ts`，不得复制 CSS border Spinner，也不得重新引入 Lottie runtime、WASM 或外部动画请求。
- `workspace-content-layout.ts` 是能力、设置、联系人等管理页面内容面的唯一入口；正文铺满可用工作面，水平留白只由 `--workspace-content-gutter` 控制，并通过 `clamp(20px, 2vw, 32px)` 随屏幕平滑增长。业务页面不得再写私有页面边距或 `max-width`。
- 共享 Surface Header、Agent 内联详情和横向滚动区也必须复用同一 gutter；滚动区需要出血时使用共享负边距组合，不得复制断点数值。
- 普通目录条目复用共享响应式网格，桌面显示三列、窄窗逐级收拢；定时任务正式看板保持四列并在宽度不足时横向滚动。
- `workspace-content-header.tsx` 是管理页正文标题、单句说明、页面动作与二级导航的唯一 Header；桌面态的标题、说明、动作和详情面包屑必须压进共享顶栏，浏览器使用 60px，macOS 由原生红灯 Y 中心推导高度，底部分隔线与侧栏品牌栏贯通。共享顶部补偿必须让 Web 标题与二级导航对齐侧栏品牌栏、让 macOS 内容避开并对齐原生控件；统一标题栏还必须参与窗口手势，并在侧栏折叠或无侧栏时通过共享横向安全区避开 traffic lights 与侧栏展开入口，不得给具体页面添加私有 padding。
- `mobile-shell-header-layout.ts` 是窄窗普通页面、Room Header 及其下缘浮层的共享几何合同；浏览器和 Windows 使用 52px 客户区高度，macOS 从宿主窗口控件中心投影实际高度。消费者只组合内容，不得再写自己的 `h-[52px]`、`top-[52px]` 或响应式 gutter。
- `panel-resize-handle.tsx` 是桌面右侧分栏的鼠标/键盘交互 owner：主键开始拖动并阻止原生选字，左右方向键按 16px 移动分隔条，Home/End 到有效最小/最大宽度；Tab 保持原生焦点顺序，平台组合键留给页面，Enter/Space 不隐式关闭面板。控件只请求像素宽度，原布局 owner 保留状态、边界和百分比换算；鼠标监听复用 `shared/lib/react/use-mouse-drag.ts`。
- 分栏用具名、可聚焦的竖向 separator，通过 controls 关联右面板，暴露有效像素范围及本地化当前宽度；未测量/固定宽度不进入 Tab 顺序，键盘焦点使用共享 ring token。无操作时 `gutter` 仍只占原有 8px 同色间距，`overlay` 保留原 12px 命中区；不增加常驻线条或拖手。
- [WAI-ARIA 的 separator 定义](https://www.w3.org/TR/wai-aria-1.2/#separator) 将可聚焦分隔条视为 widget，因此只在该具名可调 div 上说明 `no-noninteractive-element-interactions` 的局部例外，不放宽全局 lint。可关闭面板仍由其独立关闭动作拥有，不将 Enter 映射为业务页面关闭。
- 应用路由壳层归 `app/layout/`；通用布局不得组合业务 Feature。
