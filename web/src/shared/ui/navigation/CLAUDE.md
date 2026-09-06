# 选择与导航原语

- 本目录拥有跨页面一致的视图/筛选选择条样式，不拥有路由、内容面板或业务筛选状态。
- `UiBreadcrumb` 是页面层级与文件位置的唯一 DOM/箭头/字级/截断 owner；消费方只传用户可见层级、可选链接或返回命令，禁止自行拼接 `ChevronRight`、斜杠和间距。
- `UiTabs` 是历史视觉名称；当前消费者都是即时视图或筛选选择，因此 DOM 必须使用具名 `button group` 与 `aria-pressed`，不得伪装成站点 `<nav>` 或缺少 tabpanel/箭头键合同的 ARIA tablist。
- `UiTabs` 的所有内容与筛选切换统一使用纯文字优先的中性底线，不提供 `soft` 或胶囊变体；消费方提供的单项类名只挂在稳定包装节点上，响应式隐藏或收纳必须作用于该节点，不能留下空白 `gap`。有限互斥配置值和紧凑图标视图开关属于表单语义，使用 `UiSegmentedControl`。
- `UiDirectoryTabs` 是按使用类型命名的跨领域目录工具栏预设，只固定紧凑密度、自适应内容宽度和单行布局；它不得认识 Capability、Skill、Connector 或 Pairing。没有真实布局差异时不得再按业务域包一层 Tabs。
- `UiTabs` 只切换内容/筛选值，不持有会话关闭能力。Workspace 会话标签继续使用 `UiTabDismissButton`，由它持有既有命中区、危险态 hover、键盘 focus、原生 title 和关闭事件隔离；会话集合、固定和选中状态归 Workspace 所有者。
- 选择条文字统一消费共享 Typography 的 metadata 角色，字号/行高不在密度分支重复定义；页面级栏目使用中灰文字与中性底线表达当前位置，不借用品牌蓝；所有项始终保留同宽底线槽，未选中项只在 hover 时轻微加深，不增加阴影或布局位移。
- 真正页面导航使用链接与 `<nav>`；真正 tabpanel 只有在同时实现 roving focus、方向键和 `aria-controls` 后才能新增独立 primitive。
- 路由、页面状态和业务可见性由消费者投影后传入。
- `tabs.test.tsx` 证明 group、选择和跨领域目录预设；会话关闭的鼠标/键盘与命令隔离由 Workspace 标签行为测试覆盖；`breadcrumb.test.tsx` 证明 navigation、当前页、链接/返回动作与统一分隔符的真实 DOM 行为。
