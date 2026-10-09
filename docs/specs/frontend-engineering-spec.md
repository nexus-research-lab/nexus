# Web 前端工程与设计系统治理规范

范围：代码所有权、依赖方向、组件抽象、视觉系统落地、注释、测试与迁移。

视觉判断只以 [`design.md`](../../design.md) 为准；产品语法见 [`dialog-design-spec.md`](./dialog-design-spec.md)（弹窗）、[`web-surface-density-spec.md`](./web-surface-density-spec.md)（页面信息层级）、[`capability-page-design-spec.md`](./capability-page-design-spec.md)（能力页）。本文只定义这些规则如何落成代码。

## 0. 重构执行顺序

前端治理分三阶段，顺序不可颠倒：不得用第二阶段的局部视觉调整绕过第一阶段的归属治理；第三阶段反向验证前两阶段没有遗留死代码、兼容壳或无主实现。

### 第一阶段：统一实现与所有权

目标：相同语义的组件只有一个实现和样式所有者。全局字体、字号、间距、配色和密度优化留到第二阶段。

- 盘点按钮、表单、菜单、标签页、弹窗、浮层、列表行、文字层级、状态反馈和页面布局中的私有实现，按 §1 第 2 条归并；业务页只保留无法抽离的领域状态和特殊几何。
- 无法归并的原生控件或命中区，须在所属模块文档写明例外理由，并有行为测试锁定语义。
- 共享组件须同时拥有 token、状态、焦点、键盘、ARIA、主题和窄屏合同，不能只抽出一段 className。
- 消费者使用同一实现与既有语义 variant，不重写内部样式。领域局部样式仅在业务语义、交互或内容几何确有需要时允许，并写明理由、范围和行为验证；“历史如此”或“看起来更好”不是理由。

退出：存量私有实现已归并或登记例外；新代码不扩大重复；关键公共行为有测试和架构门禁。

### 第二阶段：复核规范与整体体验

以第一阶段的唯一实现为基线，复核规范本身（“已共享”不等于“设计正确”）：

- Web、macOS、Windows 宿主下的整体尺寸、窗口 chrome、安全区、页面 gutter、密度与窄窗退化，是否过大、过松或比例失衡；
- 点击目标的可见尺寸、实际命中区、间距、图标、阴影、高亮及 hover/active/focus-visible/disabled 与动效；
- 字体栈、字号、字重、行高、对比度、截断与多语言长度，层级不依赖业务文件的局部字号；
- 首次渲染、资源刷新、路由切换、弹层开关、主题切换与动画期间的跳动、频闪、重复反馈和误触；
- 功能与层级相近的模块使用相近的布局、密度和交互语法，差异须来自业务含义。

退出：规范经真实页面与典型窗口尺寸验证；问题修在共享所有者而非单页补丁；主题、语言、键盘、宿主差异与关键动态过程有可复现验收证据。

### 第三阶段：反向审计与债务清理

- 反向扫描原生 DOM 控件、任意值样式、重复常量、同义 helper、过渡适配层、无引用导出、不可达分支、失效状态和过期文档。
- 每个命中项归入“合并到公共所有者”“删除”或“有边界且有测试的例外”；不以“以后可能复用”保留无调用者代码。
- 删除或归并时同步 L3 契约、组件清单、Gallery、架构门禁和行为测试。
- 引用审计区分生产入口、生成协议与动态测试入口：只被旧测试调用的过期生产 helper 应删除，有效断言迁到当前生产入口，不为保留测试维护第二套算法；生成类型不能仅凭前端零引用删除。

退出：无未说明的页面级普通控件和私有视觉规则；无无调用者的兼容壳、导出或状态分支；lint、typecheck、构建、组件测试、架构合同及 Web/macOS/Windows 代表性页面复查全部通过。

## 1. 完成标准

1. 目录能解释所有者，import 能解释依赖方向；
2. 相同交互合同只有一个 primitive，相同跨页面几何只有一个 pattern；
3. 业务页面通过语义 Props 选择样式，不复制颜色、阴影、圆角、层级、断点或浮层几何；
4. 业务规则变化同步更新文件 `INPUT / OUTPUT / POS` 契约和所属模块文档；
5. 公共行为有自动化测试，视觉变化覆盖主题、窄屏、焦点和状态矩阵；
6. `lint`、`typecheck`、目标行为测试和前端架构门禁通过。

## 2. 代码地图与依赖方向

目标结构如下。迁移期间旧目录可保留，但新增代码按此判断所有权，不扩大历史债务。

```text
src/
├── entries/       多入口；只选择并启动 app
├── app/           Provider、Router、全局样式和应用生命周期
├── pages/         路由页面与页面级协调；不拥有可复用业务规则
├── widgets/       可独立理解的大块界面，如 ConversationPanel、WorkspaceBrowser
├── features/      用户动作与用例，如 send-message、set-goal、connect-provider
├── entities/      Agent、Room、Session、Goal、Execution 等业务资源
├── shared/        不依赖 Nexus 业务对象的 UI、transport、i18n 与通用函数
└── generated/     后端协议生成物；不承载手写业务规则
```

```text
entries -> app -> pages -> widgets -> features -> entities -> shared
                         \-----------> entities -> shared
```

上层可跳层依赖，底层不得反向 import：

- `shared` ↛ `entities / features / widgets / pages / app`；`entities` ↛ `features / widgets / pages / app`；`features` ↛ `widgets / pages / app`；`widgets` ↛ `pages / app`。
- 路由能力由 page/app 注入，或使用无业务状态的 `shared/navigation/route-paths.ts`；页面和 Feature 不为构造 URL 导入 App 装配层。
- 已迁移的 entity/feature/widget 切片之间只访问对方 `public.ts`；未迁移目录直接导入职责文件，不为命名形式新增全域 barrel。
- `web/scripts/frontend-boundaries.test.mjs` 用 TypeScript AST 检查别名、相对路径、重导出、动态与 side-effect import，没有白名单；不得通过改写 import、移到含混目录或加例外恢复向上耦合。

### 2.1 现有目录的归属

| 当前代码 | 目标所有者 |
| --- | --- |
| 中立 UI Hook（原 `hooks/ui`，已迁移） | `shared/lib/react`；非 React 剪贴板适配归 `shared/lib/browser` |
| `hooks/agent`、`hooks/conversation` | 对应 entity model 或具体 feature |
| `store/agent`、`store/conversation` | 对应 entity model |
| 应用壳状态 store | `app/model` 或对应 widget |
| `lib/api/core` | `shared/api` |
| `lib/websocket` | `shared/transport/websocket` |
| 领域 API | 对应 entity/feature 的 `api` |
| `types/generated` | `generated` |
| 其他业务 types | 对应 entity/feature 的 `model` |
| `shared/ui/workspace` | workspace widget；只留下真正无业务的原语 |
| `shared/ui/onboarding` | onboarding feature |
| `conversation/shared/feed`、`composer`、`thread` | 对应 conversation widgets |
| `conversation/shared/session`、`goal`、`execution` | 对应 entities 与用户动作 features |

按业务切片渐进迁移，不做一次性全树移动。旧路径可在一个迁移阶段保留窄兼容入口，新代码不得从旧聚合目录扩散。

## 3. 切片内部结构

```text
<slice>/
├── api/       transport 调用、DTO 与边界 mapper；不依赖 React
├── model/     类型、状态机、selector、resource hook 与 store
├── ui/        受控视图与局部交互
├── lib/       仅本切片使用的纯函数
└── public.ts  显式公共入口；禁止 export *
```

- 只在需要时建目录；少于三个紧密相关文件不建子目录。
- `controller` 只用于协调多个资源、命令或生命周期；普通组件局部状态留在组件内。
- 不用 `utils.ts`、`helpers.ts`、`common.ts` 等无所有权含义的名称。
- `model` 的投影和状态转换保持纯函数，React Hook 只绑定生命周期。
- 模型只返回业务语义，不返回 `className`、`CSSProperties`、Tailwind utility、颜色/阴影或动效时序；视图几何进入有所有者的 `*-layout` / `*-styles` recipe，控件外形与状态进入共享原语。

## 4. UI 系统分层

```text
design token -> visual recipe -> primitive -> pattern -> domain widget
```

### 4.1 Design token

入口：`web/src/app/styles/theme-tokens.css`。分类：主题基础（颜色、字体、状态色）；语义表面（surface、modal、button、input、chip）；几何（控件高度、圆角、页面 gutter、浮层 gap、视口 inset）；空间层级（sticky、menu、popover、dialog、tooltip、tour）；动效（duration、easing）。

- 业务文件不得出现 raw color、任意阴影或任意高层级。普通 Tailwind 间距刻度可用；只有跨页面须同步变化的几何才晋升为 token。
- 圆角：控件 `--radius-control-*`，独立表面 `--radius-surface-*`；相关 class 与 Input/Chip/Popover/Dialog recipe 只引用变量，内容别名指向相应尺度。Tour 高亮与 Tooltip 用公共控件圆角，不引用未定义名称或另造近似档位。
- 使用前确认 token 当前定义与语义；旧名称在消费端替换，不补兼容别名。
- 宿主或组件局部注入的可选变量必须有有效 fallback。
- 静态声明不能证明 DOM 继承或属性类型正确；运行时拼接、生成式文档和外部 CSS 仍按实际 Surface 验证。

Windows `NexusNativeTheme` 是 Web token 的平台投影，不能独立调色：当前只投影浅色主题；`native-theme-contract.test.mjs` 校验每个语义 Brush 与阴影颜色的来源，并显式保留“顶层窗口背景不透明”的宿主差异。改 Web token 须同步该投影；源码检查不代替 WPF 渲染或原生多主题验收。

### 4.2 Visual recipe

Recipe 把 token 组合成视觉语法，如 `surface-popover`、`input-shell`、`radius-control-md`。

- 通用 recipe 属 UI 基础设施；`.nexus-chat-*`、Workspace、Launcher 等领域样式归对应 widget/feature。
- 业务组件不得用 `rounded-[Npx]`、`shadow-[...]` 或 raw `color-mix` 复刻已有 recipe。同值不代表同语义：10px 须说明是 control radius 还是其他几何。
- App chrome 文字（字体、字号、行高、默认字重、tracking）由 `theme-tokens.css` 字号阶梯、`theme-recipes.css` 的 `.ui-type-*` 与 `shared/ui/typography/typography-styles.ts` 的 typed role 共同拥有。
- 业务选择 `display / featureTitle / objectTitle / pageTitle / sectionTitle / body / control / supporting / metadata / caption / code`，只负责标签、布局、截断和换行。
- 聊天、Workspace 文件、品牌字形和图形内微标签是独立 Surface，由所有者声明阅读或像素对齐理由。

### 4.3 Primitive

Primitive 同时拥有 DOM、键盘、焦点、ARIA 和视觉状态合同（Button、Input、Dialog、Popover、Menu、Tabs、Tooltip 等）。

通用：

- Props 用 `size / tone / variant / density / elevation / layer / viewport` 等有限语义；默认值可直接用于普通场景。
- `className` 只做外部布局和宽度约束，不覆盖颜色、圆角、阴影、层级、hover 或 focus。
- variant 须有真实视觉或行为差异，相同的合并。
- 普通按钮、输入和模态不得绕过 primitive 手写第二套行为。
- 业务层不得导入 primitive 内部 class 投影（`button-styles.ts`、`form-control-styles.ts`、`choice-styles.ts`、`MENU_ITEM_BASE_CLASS_NAME`）手写第二套 DOM。

按钮：

- 文字、导航链接、纯图标动作分别用 `UiButton / UiLinkButton / UiIconButton`。
- `UiButton`：`surface` 为带底色的次级动作；`outline` 为与页面同层、透明无阴影但需稳定边界的动作组；`ghost / text` 为无边界轻动作。不得用局部 `background / border / shadow` 改造变体。
- `UiListActionButton` 只在 `UiIconButton` 上组合事件隔离与可见性，不持有第二套 DOM、tone、焦点或禁用样式；按行展示用 `visibility="hover"`，共享规则保证键盘和触摸可达。
- 已有详情浮层的 IconButton 用 `tooltip={null}` 关闭自动短提示，由详情拥有 `aria-describedby`。

列表行：`UiListRow` 用 `density / variant / muted` 表达侧栏密度、连续列表/独立边界与弱化。静态行无 hover。禁用必须用 `disabled` 保留语义并阻断命令，不靠移除回调或私设透明度。

输入与字段：

- 单行、多行、原生选择分别用 `UiInput / UiTextarea / UiNativeSelect`。工作区、资料文件与记忆的源码编辑用 `UiSourceEditor`，保留原生编辑事件，不接管业务草稿、保存或快捷键。嵌入领域复合控件的无壳原生输入由该 pattern 负责。
- `UiField`（单控件）：
  - `htmlFor` 与目标控件 `id` 显式配对；
  - 公共层把可见说明加入 `aria-describedby`，把当前错误关联到 `aria-errormessage / aria-invalid`，保留调用方已有描述；
  - 多输入组不把同一字段身份复制给所有 children；分段选择等复合字段由业务提供组名和各控件名称；
  - 原生校验只定位当前表单首个可校验的无效输入，由最近 Field 显示一次；错误恢复后声明式恢复调用方最新 ARIA 属性，不用 `removeAttribute` 擦除业务校验；
  - 浏览器 validity 与业务错误是独立事实，公共层不推断业务值有效性。
- 无 `htmlFor` 的具名 `UiField` 表示复合区域：名称经 `aria-labelledby` 关联 `role=group`，说明与整组错误属于该组；不生成无目标 label，不把整组错误写到每个输入。单输入仍显式配对，复合输入保留各自名称。
- 可增删表单行：React key 用草稿生命周期内稳定的行身份，不用下标或可编辑值；行名称和移除动作区分当前项，错误仍关联原控件；纯模型只接收身份并投影草稿，新身份由视图事件创建；本地行身份不进入保存协议，不替代服务端资源身份。
- 已选实体用 `UiRemovableChip`：移除动作是具名 native IconButton；菜单触发器与移除按钮为兄弟节点，不嵌套 button，不用 `span role=button`。
- 搜索用 `UiSearchInput`，字符串标准化和字段匹配调用 `shared/ui/form/search-query.ts`。页面拥有可搜索字段、包含/前缀规则、空查询含义、筛选条件和本地/远端/跨域范围。导航侧栏不把列表筛选伪装成下探搜索；远端请求生命周期不进入 primitive。

选择与开关：

- 按钮式选择用 `UiChoiceButton`；权限范围等互斥表单选择用保留 native radio 的 `UiRadioChoice`。生成式问答等稳定领域 Widget 的原生选项按 §4.5 保留。
- 同一权限请求的多个展示实例：radio `name` 由视图实例生成，不能只用 request ID（否则两个展示区合成一个选择组）；请求身份与授权回调由原业务控制器持有，DOM 分组不参与授权；权限组用具名 Field 关联禁用说明，完整说明的权限卡分别关联标题与描述；选择不隐式触发允许/拒绝。
- 整行复选项用 `UiCheckboxRow`，公共层持有实例级名称/说明关联，业务只提供值、密度、说明和回调。
  - 默认只以可见 label 命名，description 与调用方描述合并，装饰图标不参与名称，显式 `aria-label / aria-labelledby` 优先。
  - 整行点击和 Space 由唯一 native checkbox 改值；disabled 或所属 fieldset 禁用时无命令、无 hover；不为纯属性转发加私有包装。
- 二元开关用 `GlassSwitch`：单一 native button/`role=switch` 持有 checked、键盘、焦点和真实 disabled。不在 disabled switch 外套 `span role=button` 等第二命中区；解释受保护状态时由可操作 switch 的 `onChange` 进入确认或说明。

菜单、标签页与分段控制：

- Select、Slash、多选 listbox 条目由 `SelectMenuOptionRow` 持有原生 button、`role=option`、`aria-selected` 与活动数据属性；业务只提供内容、密度、disabled 规则和选择命令。
- Action Menu 与上下文菜单行由 `UiMenuActionRow` 持有原生 button、`role=menuitem`、禁用语义、命中几何与 active/hover/focus/tone；业务只组合内容、级联和命令。
- 页面内容、目录视图和列表筛选的标签切换用只有中性底线选中态的 `UiTabs`；目录工具栏的紧凑自适应预设用跨领域 `UiDirectoryTabs`，不建 `Capability*Tabs` 等领域转发层。
- 有限互斥配置值用 `UiSegmentedControl`，不凭审美与 Tabs 互换。
  - 带可见组名用 `showLabel` 组合公共 Field，外层持有唯一 group 名称，不包原生 label 或叠加同名 group。
  - 文字角色、密度、图文高度与换行归组件；消费者只声明图标、值、命令和外部布局。
  - 纯图标选项用 UiTooltip，不叠加原生 title；受控选择用原生按钮与 `aria-pressed`，提示或焦点移动不改写值。

Dialog 与 Tooltip：

- Dialog 名称由 `UiDialogHeader.title` 自动关联最近 `UiDialogBackdrop`；完整命名合同见 [dialog-design-spec](./dialog-design-spec.md#可访问性与行为)。
- 只读 Tooltip/用量详情用 `restoreFocus: false`，开关不移动焦点；交互式菜单和 Dialog 遵守焦点归还合同。

### 4.4 Pattern

Pattern 统一多个控件在页面和窗口尺寸中的协作（Primitive 只统一一个控件），如 ResponsiveDialog、AnchoredPopover、FilterBar、SettingsSection、CatalogCard、FloatingDock，以及在一个共享边界中保留两个独立命令与焦点的 `UiSplitButton`。

- **目录筛选**：能力与联系人等目录的具名下拉筛选由 `shared/ui/menu/filter-select.tsx` 的 `UiFilterSelect` 组合紧凑 `UiSelectMenu`；固定必填文字标签，不暴露前导图标参数，不保留领域转发层。视觉结构只在 `design.md` 定义；页面拥有选项、筛选状态和按内容调整的容器宽度。普通表单选择直接用 `UiSelectMenu`。
- **目录卡片**：
  - 整卡主动作经 `WorkspaceCatalogCard.primaryAction` 声明；Article 保留内容语义，主按钮在局部隔离堆叠中位于内容下方，原生次动作独立命中；业务不复制覆盖按钮或整卡 hover/focus 配方。
  - 紧凑内容用 `size="dense"`；创建入口 `WorkspaceCatalogGhostAction` 只在 `UiButton outline` 上组合卡片尺寸与虚线边界。
  - 授权行用静态 `UiListRow` 组合唯一 `GlassSwitch`，整行或 Skill 卡片不得成为第二个切换命中区；失联但已授权的 Connector 必须仍可取消。
- **能力详情**（领域内跨子页 Pattern 留在领域 `shared`）：
  - Skill、Connector、自定义 MCP 与 WorkGraph 详情由 `CapabilityDetailPage` 持有内容轴；唯一 `CapabilityDetailHeader` 组合全站 `UiBreadcrumb` 渲染“返回目录 / 当前对象”；前导图标、标题、元数据、说明和响应式动作对齐由 `CapabilityDetailIdentity` 持有。
  - 子页不得直接引用 `WorkspaceContentDetailHeader`、手写 `objectTitle` 与动作容器、复制箭头/斜杠/间距，不得把目录态 `WorkspaceContentHeader` 用作对象身份区；详情路由不残留目录 Header 或搜索控件。
  - Workspace 文件层级同样只向 `UiBreadcrumb` 提供可见名称与相对路径段。
- **设置行**：
  - 普通二元行由 `settings/shared/settings-panel-ui.tsx` 的 `SettingsToggleRow` 组合唯一 GlassSwitch，标题即可访问名称，实例级说明 ID 经 `aria-describedby` 关联，行与说明不加点击命令。
  - Preferences、Echo、运行偏好的 checked、禁用条件和回调归调用方。
  - Browser 权限卡、模型表单内联开关和授权列表保留领域布局，直接复用 GlassSwitch 的说明关联，不为统一布局吞掉确认、恢复或提交边界。

### 4.5 Domain widget

Widget 可认识 Agent、Room、Goal 等产品对象，但只组合下层合同，不重定义基础视觉。

- **原生交互节点**：仅当 DOM 命中区本身表达图形几何时保留（如 WorkGraph 边中点、节点卡、折叠计数）；原生 button 能表达时不得用 `div role=button` 加手写键盘事件。缩放、搜索、定位、关闭、保存等标准动作用 UiButton / UiIconButton；画布内浮动工具条和搜索面复用语义 Surface。
- **WorkGraph**：
  - `ExecutionGraphInspector` 持有节点/连线详情的外壳、标题、关闭动作和滚动区，复用 `surface-popover`、Typography 与 `UiIconButton`；它是只读非模态检查器，不另建 Portal、模态锁或执行状态。
  - 画布只负责精确选择身份、定位和逆缩放；详情保持屏幕尺寸。
  - 顶栏生命周期、部分投影和旧快照提示用 `UiBadge` 的 tone/size/shape；详情内运行活动是静态 `UiListRow`。
  - `NamedWorkGraphSketch` 持有缩略图的拓扑列、连线、节点和终态微标签几何；外围 Surface 与普通文字共享。
- **Access（Login/Setup）**：
  - 共用 `features/access/AccessPageFrame` 与 `AccessPageIntroduction`，只拥有品牌背景、Logo、宣传标题和响应式两栏；宣传标题属品牌 Surface，尺度归同一配方。
  - 单列凭证与并排初始化字段用 regular/wide 表单宽度；普通文字与表单用 Typography、Panel、Field、Input、Button。
  - 不复制背景或表单材质；认证状态和初始化命令不进入该展示 Pattern。
  - 装饰背景上的独立表单用 `UiPanel variant="filled"`（只由 Panel 消费面板背景 token）；透明 card、虚线和无壳 plain 各有用途，消费者不另加背景和阴影。
- **Conversation**：
  - Composer 浮动工作栈属 conversation widget，不为复用 DM/Room 放进 `shared`。Composer 间距配方（含待发送队列）只在视图消费 `composer-styles`，不经 controller/model 返回 CSS。
  - Composer 附件预览与移除组合 `UiButton / UiIconButton`，保留独立兄弟命中区；领域只拥有缩略图几何、文件与草稿作用域；图片角移除用共享 micro 尺寸。Chip、输入壳与 Composer 聚焦壳的圆角只由共享 recipe 定义。
  - 私域目录预览与时间线复用 filled Panel 与 Typography；事件视图只保留方向表达与 Markdown 正文；头像叠放/溢出计数归身份图形。
  - User 消息编辑器组合公共输入壳、按钮与原生无壳 textarea；原位正文测量和独立 Footer 属该编辑器。Composer 与消息编辑共用中立 IME 事件识别，组合生命周期和快捷键策略留在各自边界。
  - User/Assistant 正文尺度与外侧节奏唯一归 `message-reading-layout`，数据投影不带布局字段；文件卡片同样分离 exact 路径/Agent/动作投影与阅读几何。
  - 文件预览与外部动作独立：缺预览 handler 不禁用有效的下载/显示动作；切换全局 Agent 不改写显式文件来源。
- **生成式结构化问答**：选项行可保留原生 `fieldset`、radio/checkbox 与内嵌无壳 textarea（命中区与选择标记共同表达题目几何）；拒绝、提交仍用 `UiButton`，题目、说明、提示和终态摘要仍用 Typography role。

`features/pages` 原生 button 唯一例外清单（路径相对 `web/`）。数量是当前事实，不是额度；架构门禁用 TypeScript AST 检查 JSX 与 `createElement`，增删或迁移须同步所有者、几何理由和行为验证。

<!-- native-button-owners:start -->
| 所有者 | 原生节点数 | 几何理由 |
| --- | --- | --- |
| `src/features/conversation/room/surface/mobile/room-mobile-conversation-switcher.tsx` | 1 | 顶栏下拉 Sheet 的整面 underlay 关闭热区；模态行为仍归共享 Dialog。 |
| `src/features/conversation/shared/execution/execution-workgraph-canvas.tsx` | 3 | 工作图边中点、节点卡和折叠计数的坐标命中区。 |
| `src/features/conversation/shared/message/agent-mention-chip.tsx` | 1 | 随 Markdown 行内字号排布的 Agent 身份与 handoff 实体。 |
| `src/features/conversation/shared/message/blocks/artifact/file/file-artifact-block.tsx` | 1 | 完整文件产物卡的打开热区；文件命令仍由产物领域持有。 |
| `src/features/conversation/shared/message/blocks/artifact/image/image-block.tsx` | 1 | 原图等比缩略图的预览热区。 |
| `src/features/conversation/shared/message/item/view/content/content-system-event.tsx` | 1 | 时间线系统重试行的原位展开，保留锚点与倒计时几何。 |
| `src/features/conversation/shared/message/item/view/user/message-user-attachments.tsx` | 1 | 用户消息中的图片/文件整体预览入口。 |
| `src/features/conversation/shared/message/ui/message-avatar.tsx` | 1 | 头像轮廓本身的详情热区；图像与交互尺寸一致。 |
| `src/features/conversation/shared/scroll-to-latest-button.tsx` | 1 | 浮动滚动入口的 44px 热区包围较小状态芯片；DM/Thread 共用同一所有者。 |
| `src/features/conversation/shared/session-navigator/conversation-session-navigator.tsx` | 2 | 连续轮次刻度与整张预览卡的导航热区。 |
| `src/features/launcher/hero/launcher-hero-stage.tsx` | 2 | 随舞台整体缩放的品牌复合入口和发送角色图像热区；普通最近入口使用 UiButton。 |
| `src/features/launcher/hero/pile/launcher-agent-token.tsx` | 1 | 物理舞台中的可拖动 Agent token，位置和热区由场景共同计算。 |
| `src/features/navigation/sidebar/view/sidebar-rail-action.tsx` | 1 | 主导航和固定会话共用的 Dock 轨道入口，容纳计数、拖放与当前态几何。 |
<!-- native-button-owners:end -->

## 5. 抽象与晋升规则

| 重复事实 | 抽象位置 |
| --- | --- |
| 同一颜色、圆角、阴影、层级或关键尺寸 | Token |
| 多个视觉 class 总是共同出现 | Recipe |
| DOM、交互、键盘与 ARIA 相同 | Primitive |
| 响应式布局、浮动几何或组件组合相同 | Pattern |
| 业务对象和业务状态相同 | Entity/Feature/Widget |

默认从业务局部实现开始，第二个消费者出现时比较差异；跨两个领域出现第三个稳定消费者，或交互/可访问性必须全局一致时，才晋升到 shared。单消费者透传 wrapper、只为缩短 className 的组件和假想复用不得晋升。

## 6. 视觉与交互治理

### 6.1 阴影

阴影表达空间高度，不表达重要性：

- 普通 button、nav row、panel、card 无阴影；primary action 用行动色而非阴影；selected/current 用中性背景、文字或位置。
- menu/popover、dialog 与真正悬浮的 floating action 用对应 elevation。
- 业务代码不得写 `shadow-[...]`；`features/pages` 任意阴影基线为零，由 `frontend-foundation-contract.test.mjs` 禁止，无逐文件额度。

### 6.2 状态

`hover / active / selected / pressed / primary / focus-visible / running` 是不同语义，即使颜色接近也用不同 token/recipe。颜色不得是状态的唯一信号。

### 6.3 浮层与小窗口

- **定位**：anchored overlay 的 gap、viewport inset、min/max 宽高、碰撞、翻转、滚动跟随和 Portal 归共享定位层。业务只选命名 layout preset、`placement / align / layer` 与真实内容高度估算，不导入底层定位模型，不提交 `gap / viewportMargin / minWidth / minHeight / maxHeight`。
- **层级**：z-index 只经语义 layer（`UiDialogBackdrop.layer` 或 `shared/ui/overlay/layer-styles.ts`），禁止加整数解决遮挡；`features/pages` 数字 z-index 基线为零（同上门禁）。嵌套 modal 顺序由 modal stack 负责。
- **Dialog 视口**：桌面限高、窄屏 inset、固定 header/footer 与 body scroll 归 viewport variant，经 `UiDialogBackdrop.inset` 与 `UiDialogShell.viewport` / `UiDialogFormShell.viewport` 选择。
  - 选择器和短向导 `compact`；自然高度紧凑目录 `compactMax`；长表单 `adaptive` / `adaptiveMax`；图片/短文本查看 `visualPreview` / `documentPreview`；大型图形/对照工作台 `workbench`。
  - 不得复制 `82dvh / 760px / 16px` 等视口公式，也不按内容量发明相近像素高度。
- **Dialog 宽度**：只经 `size` 选择，Shell 上禁止补写 `max-width / vw`；档位不合适时先判断是否为可跨业务复用的新内容类型。单行命名/创建类 Prompt 用紧凑决策宽度，多行输入提升一档；Prompt 的 Header、Input 与确认动作由共享 Decision Dialog 统一，不以局部宽高或私有按钮修补。
- **反馈**：全局 feedback 复用 popover 材质、`feedback` layer、Typography 与共享 Button；内容流提示由 `UiInlineNotice` 的 `full / compact` 档位拥有列宽与阅读宽度。业务只提供已确认的标题、影响、下一步和至多一个动作，不以局部宽度、阴影、圆角或 z-index 抬高反馈。

### 6.4 响应式

- 仅布局变化用 CSS media/container query，组件自身宽度决定的布局优先 container query；只有行为变化才用 `useMediaQuery`。
- 产品断点经共享语义入口使用，不新增近似断点。
- 窄屏不建第二套主题或组件，只改变密度、排列和导航呈现。

## 7. 注释与文档合同

业务入口、状态机、协议 mapper、复杂 hook 和跨文件基础组件使用三行文件契约：

```ts
// INPUT: 接受的可信事实、上游资源或用户动作。
// OUTPUT: 对外产生的视图、命令、状态或副作用。
// POS: 在模块中的唯一职责，以及明确不负责的内容。
```

- 修改输入、输出、所有权或副作用时同步契约。
- 注释解释为什么、边界和失败语义，不复述函数名或 JSX；导出的复杂类型/函数仅在签名无法表达约束时写 TSDoc。
- 每个 entity/feature/widget 根目录最多一份职责文档；只有独立状态机或协议边界才加子目录文档。
- 文档写稳定不变量；进行中的迁移计划标记 `non-normative`。
- 代码、测试和文档冲突时，不得只改一相就结束。

`web/scripts/frontend-file-contract.test.mjs` 对 `shared/ui/button`、`dialog / form / list / menu / navigation / overlay / typography / workspace/catalog` 及共享 `lib`、`navigation` 递归强制契约：识别首条代码前的真实注释，拒绝缺项、重复、空内容与占位文本。

其他领域随迁移逐批纳入；未纳入不是省略边界说明的理由，也不写机械模板凑覆盖率。

## 8. 测试合同

| 层级 | 必测内容 |
| --- | --- |
| Token/Recipe | 语义入口存在、禁止值不再新增、主题映射完整 |
| Primitive | DOM、真实键盘事件、焦点、ARIA、disabled 与状态组合 |
| Entity model | mapper、selector、状态机、recovery 与 stale response fence |
| Feature | 一次用户动作从输入到 command/result 的完整状态流 |
| Widget | 关键组合状态、窄屏结构和资源失败降级 |
| Page | 路由、恢复与少量主路径浏览器 smoke test |

- 源码正则只作架构或禁止项门禁，不替代行为测试，也不冻结公共控件调用方的实现细节；特殊表面组合由设计规范、行为测试和浏览器验收判断。
- 涉及布局、Portal、碰撞和视口尺寸的 UI 必须用真实浏览器验证。

### 8.1 测试入口与命令

| 入口 | 合同 |
| --- | --- |
| `src/**/*.test.tsx` | 与 primitive/pattern 共置的 Vitest + jsdom；经 Testing Library 从角色、名称和真实用户事件观察 |
| `scripts/*.test.mjs` | 纯模型、协议、架构边界和禁止项；不伪造 DOM 交互结论；有界并发运行，避免大量 Vite 转换进程使门禁随机崩溃 |
| `npm run test:components` | 日常组件测试，默认排除 `src/dev/ui-gallery` |
| `npm run test:contracts` | 规范、边界、恢复和禁止项合同 |
| `npm run test:ui-gallery` | 仅 Gallery DOM 陈列合同 |
| `npm run test:components:all` | 显式覆盖全部 Vitest 文件 |
| `npm test` / `npm run test:standard` | 串行覆盖 components 与 contracts |
| `npm run check` / `check:standard` | 串行 lint、typecheck、日常测试与生产构建；不启动浏览器，不含截图或视口矩阵 |
| `npm run test:browser:smoke` | `ui-gallery.spec.ts` 的 `light-zh-1440` 项目，单 worker，不依赖业务后端 |
| `npm run test:browser:full` | 固定版本 Playwright + 独立端口与优化缓存的 Vite 服务器，完整浏览器合同 |
| `npm run check:ui` / `make check-web-ui` | 日常门禁 + Gallery DOM 合同 + 完整浏览器矩阵 |
| `make check-web` | 仅标准前端门禁 |

- Vitest 默认两个 fork worker，可用 `VITEST_MAX_WORKERS` 覆盖。
- 浏览器服务器固定 `browser-test` mode，避免并发开发或 SSR 检查使缓存失效。首次安装 `npx playwright install chromium webkit`，Linux CI 加 `--with-deps`。
- `frontend-token-contract.test.mjs` 用既有 CSS 工具链与 TypeScript AST 检查全部生产 CSS/TS 的静态 `var()`、Tailwind 简写和模板 CSS：
  - 必需引用须有声明，可选注入须有 fallback；
  - 三主题 canonical 别名不得缺失、循环或在同一声明块重复；
  - 进入 `color-mix()` 的公共控件颜色槽须解析别名并通过 DOM CSS 解析器颜色校验（拒绝把渐变当颜色）；
  - 不执行运行时表达式，不证明 DOM 继承、实际绘制或对比度；无逐文件额度。
- `.github/workflows/frontend-check.yml` 对前端、规范与 Windows 原生主题变更运行同一套检查；失败不得靠跳过、重试或更新截图消除。
- macOS 原生宿主（需图形会话，不进默认门禁，不替代完整业务或 Windows 检查）：
  - `make app-check-ui`：在已解锁的图形会话编译独立 QA App，复用生产窗口与 WKWebView 源码，验证 Gallery 的原生输入、浮层、焦点及隐藏恢复。
    - 应用标识、偏好、状态根、端口和优化缓存隔离；不启动产品 sidecar，不以旧安装包作当前源码证据。
    - 源码清单、日志、报告与截图按运行保存，详见 `desktop/macos/README.md`。
  - `make app-check-ui-app`：同一宿主验证真实 Launcher/工作台、响应式导航、Header 双击缩放与恢复。
    - 读取与空闲订阅由隔离夹具提供，Vite HTTP/WS 代理禁用；`native-ui-fixtures.test.mjs` 用真实本地服务验证读写边界与零转发。
    - 锁屏或无图形会话不能用合成 DOM 事件代替原生输入证据。

### 8.2 视觉回归矩阵与 Gallery

视觉回归至少覆盖：light / dark / rain；320px、产品窄屏断点附近与桌面宽度；default / hover / focus / disabled / selected / loading / error；中英文长文案与 reduced motion。

Gallery 入口 `http://localhost:3000/ui-gallery.html?theme=light&locale=zh`：

- 独立开发 HTML，不经登录态、业务 API 或产品路由；不得加入 Vite 生产 `rollupOptions.input`。
- `theme=light|dark|rain`、`locale=zh|en`、`section=foundation|content|interaction|workspace|coverage` 必须写回 URL，保证复查地址可复现。
- 直接渲染真实 `shared/ui` 组件，不建截图替身。
- 公开 React 组件必须进入唯一覆盖清单：可视组件直接渲染，复合组件内部原语由真实父组件覆盖，Provider、SVG Filter 等无界面基础设施标注真实消费路径。新增公开组件未登记时覆盖合同失败。

### 8.3 浏览器测试

`browser-tests/ui-gallery.spec.ts` 是浏览器行为真相入口：

- 矩阵：Chromium 为 light/dark/rain × 中/英 × 320/767/768/1440px；WebKit 为同主题/语言 × 320/1440px。
- 覆盖：控件实际高度/字号/字重/图文间距；按钮禁用/忙碌；真实 focus-visible；hover 几何；选择器/动作菜单键盘与禁用项；碰撞翻转与滚动重定位；模态滚动锁、初始焦点、Tab 循环、弹窗内浮层命中、逐层 Escape、焦点归还；正常/减少动效。
- 受控 Workspace 标签：选择、创建、固定与键盘关闭后的活动项恢复。Room 持久化与最终替换规则由导航功能共置测试验证（§8.5）。
- 表单：实际输入、原生选择、搜索清除与多行编辑；技术字段的等宽字体、验证码居中命中区与前导零保留；原生 FormData 验证展示角色不改写提交值。
- 键盘遍历用真实宿主规则：macOS WebKit 用 Option-Tab，其他用 Tab；不改用户系统偏好，不伪造 DOM 焦点顺序。
- 模态焦点链由键盘触发验证，关闭后回到键盘触发器；鼠标触发遵循宿主原生 button 聚焦行为，不强改 macOS 点击规则（依据：[Apple Safari 键盘说明](https://support.apple.com/guide/safari/cpsh003/mac)、[WebKit 鼠标焦点说明](https://bugs.webkit.org/show_bug.cgi?id=236322)）。
- 使用真实角色/名称和浏览器布局，不以复制样式或 stub 组件伪造通过。

其他浏览器入口（同一矩阵）：

- `browser-tests/login.spec.ts`：真实登录路由 + 隔离认证响应，验证提交、未知结果阻塞、只读恢复与禁用部署。远端聊天 CJK 字体被阻断，截图只验收 App 本地字体，不得据此声称远端字体或聊天阅读面已验收。
- `browser-tests/app-shell.spec.ts`：真实 App 入口 + 固定只读快照与本地事件连接，验证 Launcher 输入、导航往返、完整侧栏标签、窄屏工作区切换，以及同账号多页面保存、刷新后的固定与取消固定。
  - 播放器 CDN 请求由已安装 WASM 响应，业务写入全部拒绝；不能据此宣称真实业务数据、第三方 CDN 或原生窗口手势已验证。

证据与 CI：

- 截图随 HTML report 输出到 `playwright-report/`；失败 trace/截图输出到 `test-results/`，CI 保留 14 天。它们是复查证据，不是跨平台像素基线门禁。
- CI 按 `--shard` 拆为十二个任务，保留全部用例，分别上传 `frontend-browser-evidence-<shard>`；`frontend` 汇总门禁只在代码检查及全部分片成功后通过，失败、取消或跳过均不算成功。
- 提交说明写明实际检查的浏览器、主题、宽度和状态。Chromium/WebKit 自动化不等于 macOS/Windows 原生窗口 chrome 或完整业务页面已验收，这些按变更范围在实际宿主复查。

### 8.4 Gallery 夹具覆盖

| 夹具 | 验证内容 |
| --- | --- |
| 目录卡片 | 真实坐标下的整卡命中、独立次动作和键盘主动作 |
| 列表次动作 | hover/focus 可见性；disabled 时无 hover 反馈 |
| Select | 浏览器测量紧凑与大号字段高度；业务经 `size` 选择，不加局部高度或阴影 |
| Agent 高级设置 | 实际权限行与 Skill 卡片：开关独立命中、失联授权撤销、锁定/提交中不可变更、长页面逐行滚动可达（不要求整页入视口） |
| Composer 附件 | 真实本地 File：图片/纯文本预览、长内容换行、键盘触发后焦点归还、exact 附件移除 |
| 主题矩阵 | 当前根元素的实际 CSS 变量无空解析结果 |
| Tour | 真实锚点；测量高亮圆角与外扩几何；Escape 与目标点击均关闭导览，目标原命令只执行一次 |
| WorkGraph | 真实画布：节点/连线精确身份、原 Agent 文件动作、键盘关闭、节点与边详情的共同不透明表面、缩放后详情保持屏幕字号与宽度；画布与详情可滚动到达，不等于所有节点同时入视口 |
| 私域/消息 | 真实时间线与消息视图：两种密度、方向与元信息；编辑聚焦、普通 Enter、IME 期间不发命令、取消重置、原 round 唯一提交；User/Assistant 实际正文尺度、外侧间距与文件文案；文件动作只更新本地观察值 |

共置 DOM 测试另覆盖：Composer 附件的 Session 切换只关闭预览、Object URL 释放、表单内默认按钮不提交、预览开关不写入持久草稿且不上传；消息的原 Agent 预览/下载隔离、Goal 控制记录不可编辑、兼容 IME 键码与空值/未变化草稿。合成事件不是操作系统输入法验收。

### 8.5 Room 导航偏好（由共置测试锁定）

- 偏好在同一持久快照中绑定 owner；每次保存先读同 owner 最新值，再应用选择、固定、排序或移除命令，不以旧页面整份内存覆盖。
- 新建成功、历史选择和明确路由选择只追加精确 Conversation；普通关闭只移除精确目标并保留固定偏好。
- 列表缺项可能来自旧快照，展示过滤不回写为标签删除；会话删除由显式删除命令清理。标签按创建时间排序。
- 最后标签替换事务只从最新集合移除原目标并加入替代项，保留其他页面打开的标签；运行时不暴露完整集合覆盖命令。
- 存储事件只表示失效：接收页重读当前快照、不回写；未变化的导航保持原状态引用。
- App 独占 owner 受理与校验，Store 不导入认证装配层。旧无绑定记录仅在既有 owner 迁移检查后认领；其他 owner 的已绑定快照在认证绑定前后都不可恢复。
- 跨页面身份失效时先清空本页并阻止迟到命令，不写空共享快照；下一次权威身份绑定只恢复相同 owner 的记录；确认登出才清除持久记录。

## 9. Agent 修改流程

除满足 §1 外：

1. 先定位所有者与现有 primitive/pattern，不从页面搜索结果复制实现；
2. 改公共视觉前列出受影响消费者，判断应改 token、recipe、primitive 还是 pattern；
3. 业务页需要覆盖公共视觉时，先证明是新的稳定 variant，而非添加任意 class；
4. 同步唯一规范，添加与变更层级匹配的测试（§8）；
5. 迭代期间跑目标测试，交付前跑 `npm run check`；
6. UI 改动检查窄屏、三主题、键盘焦点和叠层关系；
7. 用户可见变化同步 `CHANGELOG.md`。

## 10. 迁移阶段（non-normative）

1. 基础门禁：冻结新的反向依赖、任意高 z-index、重复 dialog viewport 和公共组件视觉覆盖；
2. Primitive 收口：Button、Form、Dialog、Overlay、Menu、Tabs 补齐语义 API 与行为测试；
3. Pattern 收口：ResponsiveDialog、AnchoredPopover、FilterBar、SettingsSection、Conversation 浮动工作栈；
4. 所有权迁移：按切片迁移 `hooks / store / types / lib/api`，拆分 `conversation/shared`；
5. 视觉回归：陈列面、浏览器矩阵与 CI 截图证据已建立；跨平台像素基线和原生宿主整体审查按环境渐进；
6. 清债：移除兼容导入、闲置组件、无差异 variant 和过细目录文档，开启强制门禁。

迁移状态不改变上文规范；未迁移的旧代码是已知债务，不是复制的先例。
