# Web 前端工程与设计系统治理规范

本文定义 Nexus Web 前端的代码所有权、依赖方向、组件抽象、视觉系统、注释、测试与迁移合同。它回答“代码应放在哪里、为什么能复用、修改后应影响哪些界面、如何证明没有产生第二套实现”。

视觉判断以 [`design.md`](../../design.md) 为唯一入口；弹窗、页面信息密度和能力页面的产品语法分别由 [`dialog-design-spec.md`](./dialog-design-spec.md)、[`web-surface-density-spec.md`](./web-surface-density-spec.md) 与 [`capability-page-design-spec.md`](./capability-page-design-spec.md) 定义。本文不重复这些产品规则，只定义它们如何落成可维护代码。

## 0. 重构执行顺序

整体前端治理必须按以下三个阶段进行，不得用第二阶段的局部视觉调整绕过第一阶段的组件归属治理；第三阶段负责反向验证前两阶段没有遗留死代码、兼容壳或失去所有者的实现。

### 第一阶段：统一实现与所有权

- 盘点按钮、表单、菜单、标签页、弹窗、浮层、列表行、文字层级、状态反馈和页面布局中的私有实现；
- 相同交互合同归并到唯一 primitive，跨页面的相同几何归并到唯一 pattern，业务页只保留无法抽离的领域状态和特殊几何；
- 无法归并的原生控件或命中区必须在所属模块文档中说明为什么是例外，并用行为测试锁定语义；
- 共享组件必须同时拥有 token、状态、焦点、键盘、ARIA、主题和窄屏合同，不得只抽出一段 className。

第一阶段的退出条件是：存量私有实现已归并或有明确例外记录，新代码不再扩大重复实现，关键公共行为已有测试和架构门禁。

### 第二阶段：复核规范与整体体验

- 以第一阶段得到的唯一实现为基线，重新判断统一规范本身是否合理，不把“已经共享”误当成“设计已经正确”；
- 同时验收 Web、macOS 和 Windows 宿主下的整体尺寸、窗口 chrome、安全区、页面 gutter、内容密度和窄窗退化，判断 App 整体及局部控件是否过大、过松或比例失衡；
- 系统检查按钮与其他点击目标的可见尺寸、实际命中区、间距、图标、阴影、高亮、hover、active、focus-visible、disabled 和动效；
- 检查字体栈、字号、字重、行高、对比度、截断与多语言长度，保证信息层级清楚而不依赖业务文件的局部字号；
- 检查首次渲染、资源刷新、路由切换、弹层开关、主题切换与动画期间的跳动、频闪、重复反馈和误触；
- 功能相近、信息层级相近的模块必须使用相近的布局、密度和交互语法，差异必须来自明确业务含义，不得来自历史页面各自实现。

第二阶段的退出条件是：统一规范经真实页面与典型窗口尺寸验证，发现的问题修复在共享所有者而非单页补丁中，并且主题、语言、键盘、宿主差异与关键动态过程均有可复现验收证据。

### 第三阶段：反向审计与债务清理

- 从原生 DOM 控件、任意值样式、重复常量、同义 helper、过渡适配层、无引用导出、不可达分支、失效状态和过期文档反向扫描整个前端；
- 每个命中项必须明确归入“合并到公共所有者”“直接删除”或“记录为有边界且有测试的例外”，不得以“以后可能复用”为理由保留无当前调用者的代码；
- 删除或归并实现时同步更新 L3 契约、组件清单、Gallery、架构门禁和行为测试，避免代码与说明再次分叉。

第三阶段的退出条件是：不存在无说明的页面级普通控件和私有视觉规则，不存在无调用者的兼容壳、导出或状态分支；前端 lint、typecheck、构建、组件测试、架构合同以及 Web/macOS/Windows 代表性页面复查全部通过。

## 1. 完成标准

前端改动只有同时满足以下条件才算完成：

1. 目录可以解释代码所有者，import 可以解释依赖方向；
2. 相同交互合同只存在一个 primitive，相同跨页面几何只存在一个 pattern；
3. 业务页面通过语义 Props 选择样式，不复制颜色、阴影、圆角、层级、断点或浮层几何；
4. 业务规则变化同步更新文件 `INPUT / OUTPUT / POS` 契约和所属模块文档；
5. 公共行为有自动化测试，视觉变化覆盖主题、窄屏、焦点和状态矩阵；
6. `lint`、`typecheck`、目标行为测试和前端架构门禁通过。

## 2. 代码地图与依赖方向

目标代码地图如下。迁移期间现有目录可以保留，但新增代码必须按此判断所有权，不得扩大历史债务。

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

允许的依赖方向：

```text
entries -> app -> pages -> widgets -> features -> entities -> shared
                         \-----------> entities -> shared
```

上层可以跳过中间层依赖更底层；底层不得反向 import 上层。特别是：

- `shared` 不得 import `entities / features / widgets / pages / app`；
- `entities` 不得 import `features / widgets / pages / app`；
- `features` 不得 import `widgets / pages / app`；
- `widgets` 不得 import `pages / app`；
- 页面路由能力由 page/app 注入，或通过无业务状态的共享 route contract 使用；
- 跨切片依赖只访问对方的 `public.ts`，不得穿透内部目录。

### 2.1 现有目录的归属

| 当前代码 | 目标所有者 |
| --- | --- |
| `hooks/ui` | `shared/lib/react` |
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

迁移必须按业务切片渐进完成，不提交一次性全树移动。旧路径可以在一个迁移阶段保留窄兼容入口，但新代码不得继续从旧聚合目录扩散。

## 3. 切片内部结构

Entity、Feature 与 Widget 只在需要时使用以下目录：

```text
<slice>/
├── api/       transport 调用、DTO 与边界 mapper；不依赖 React
├── model/     类型、状态机、selector、resource hook 与 store
├── ui/        受控视图与局部交互
├── lib/       仅本切片使用的纯函数
└── public.ts  显式公共入口；禁止 export *
```

- 少于三个紧密相关文件时不创建子目录；
- `controller` 只用于协调多个资源、命令或生命周期的复杂流程；
- 普通组件的局部状态留在组件内，不为了形式拆出 controller；
- 避免 `utils.ts`、`helpers.ts`、`common.ts` 等无法表达所有权的名称；
- `model` 中可测试的投影和状态转换必须保持纯函数，React Hook 只负责绑定生命周期。

## 4. UI 系统分层

UI 实现固定分为五层：

```text
design token -> visual recipe -> primitive -> pattern -> domain widget
```

### 4.1 Design token

Token 是跨主题、跨组件的值真相，当前入口是 `web/src/app/styles/theme-tokens.css`。Token 分为：

- 主题基础：颜色、字体、状态色；
- 语义表面：surface、modal、button、input、chip；
- 几何：控件高度、圆角、页面 gutter、浮层 gap、视口 inset；
- 空间层级：sticky、menu、popover、dialog、tooltip、tour；
- 动效：duration 与 easing。

业务文件不得出现 raw color、任意阴影或任意高层级。普通 Tailwind 间距刻度可以继续使用；只有跨页面必须同步变化的几何才晋升为语义 token。

### 4.2 Visual recipe

Recipe 把 token 组合成可复用视觉语法，例如 `surface-popover`、`input-shell`、`radius-control-md`。通用 recipe 位于 UI 基础设施；`.nexus-chat-*`、Workspace、Launcher 等领域样式归对应 widget/feature，不进入通用主题配方。

业务组件不得使用 `rounded-[Npx]`、`shadow-[...]` 或 raw `color-mix` 复刻已有 recipe。同值不代表同语义：10px 必须说明它是 control radius 还是其他几何。

App chrome 的字体、字号、行高、默认字重与 tracking 由 `theme-tokens.css` 的字号阶梯、`theme-recipes.css` 的 `.ui-type-*` 配方和 `shared/ui/typography/typography-styles.ts` 的 typed role 共同拥有。业务组件选择 `display / featureTitle / objectTitle / pageTitle / sectionTitle / body / control / supporting / metadata / caption / overline / code`，只自行负责 HTML 标签、布局、截断和换行；不得在每个文件重新拼一套相同文本角色。聊天、Workspace 文件、品牌字形和图形内微标签是显式独立 Surface，必须由其所有者声明阅读或像素对齐理由。

### 4.3 Primitive

Primitive 同时拥有 DOM、键盘、焦点、ARIA 和视觉状态合同，例如 Button、Input、Dialog、Popover、Menu、Tabs、Tooltip。

- Props 使用 `size / tone / variant / density / elevation / layer / viewport` 等有限语义；
- 默认值必须能直接用于普通业务场景；
- `className` 只用于外部布局和宽度约束，不得覆盖颜色、圆角、阴影、层级、hover 或 focus；
- 业务文字、导航链接和纯图标动作必须分别渲染 `UiButton / UiLinkButton / UiIconButton`；`button-styles.ts` 是 shared primitive 的实现细节，业务层不得借其 class 投影手写第二套 DOM；
- `UiButton surface` 表达带底色的次级动作，`outline` 表达与页面同层、透明无阴影但需要稳定边界的动作组，`ghost / text` 表达默认无边界的轻动作；业务页不得用局部 `background / border / shadow` 把一种变体临时改造成另一种；
- 普通单行、多行和原生选择字段必须分别渲染 `UiInput / UiTextarea / UiNativeSelect`；业务层不得导入 `form-control-styles.ts` 复制输入壳，嵌入领域复合控件的无壳原生输入由其 pattern 明确负责；
- 按钮式选择统一使用 `UiChoiceButton`，权限范围等互斥表单选择统一使用保留 native radio 的 `UiRadioChoice`；业务层不得导入 `choice-styles.ts` 手写第二套 DOM，生成式问答等稳定领域 Widget 的原生选项按其独立合同保留；
- 二元开关统一由 `GlassSwitch` 的单一 native button/`role=switch` 持有 checked、键盘、焦点和真实 disabled；业务不得在 disabled switch 外套 `span role=button` 等第二命中区，需要解释受保护状态时由可操作 switch 的 `onChange` 进入业务确认或说明；
- 标签输入和多选字段中的已选实体统一使用 `UiRemovableChip`；移除动作必须是具名 native IconButton，复合字段的菜单触发器与移除按钮必须为兄弟节点，不得嵌套 button 或用 `span role=button` 绕过合法 DOM；
- 搜索入口统一渲染 `UiSearchInput`，客户端字符串标准化和字段匹配统一调用 `shared/ui/form/search-query.ts`；具体页面仍拥有可搜索字段、包含/前缀规则、空查询含义、资源筛选条件和本地/远端/跨域搜索范围。导航侧栏不得把当前列表筛选伪装成下探搜索，远端请求生命周期也不得进入 UI primitive；
- Select、Slash 和多选 listbox 的条目统一由 `SelectMenuOptionRow` 持有原生 button、`role=option`、`aria-selected` 与活动数据属性；业务层只提供行内容、密度、disabled 规则和选择命令，不得把共享菜单 class 重新拼成第二套 option DOM；
- Action Menu 与业务上下文菜单的行统一由 `UiMenuActionRow` 持有原生 button、`role=menuitem`、禁用语义、命中几何与活动/hover/focus/tone 状态；业务层只组合菜单内容、级联关系和命令，不得导入 `MENU_ITEM_BASE_CLASS_NAME` 手写 `menuitem`；
- 页面内容、目录视图和列表筛选的标签切换统一使用只有中性底线选中态的 `UiTabs`；目录工具栏的紧凑、自适应宽度预设使用按类型命名的跨领域 `UiDirectoryTabs`，不得创建 `Capability*Tabs` 等业务域转发层。有限互斥配置值使用 `UiSegmentedControl`，不得在两者之间仅凭局部审美互换；
- variant 必须存在真实视觉或行为差异；完全相同的 variant 合并；
- 普通按钮、输入和模态不得绕过已有 primitive 手写第二套行为。

### 4.4 Pattern

Pattern 统一跨页面的结构、响应式几何或交互组合，例如 ResponsiveDialog、AnchoredPopover、FilterBar、SettingsSection、CatalogCard、FloatingDock，以及在一个共享边界中保留两个独立命令与焦点的 `UiSplitButton`。

Pattern 与 Primitive 的区别是：Primitive 统一一个控件；Pattern 统一多个控件如何在页面和窗口尺寸中协作。

领域内跨子页重复的 Pattern 留在该领域 `shared`：例如 Skill、Connector、自定义 MCP、Loop 与 WorkGraph 详情统一由 `CapabilityDetailPage` 持有内容轴，并由唯一 `CapabilityDetailHeader` 组合全站 `UiBreadcrumb` 渲染“返回目录 / 当前对象”；Workspace 文件层级也只向 `UiBreadcrumb` 提供用户可见名称与相对路径段。导航下方的前导图标、标题、元数据、说明和响应式动作对齐统一由 `CapabilityDetailIdentity` 持有。业务子页不得直接引用底层 `WorkspaceContentDetailHeader`、手写 `objectTitle` 与动作容器、复制箭头、斜杠或间距，也不得把目录态 `WorkspaceContentHeader` 复用成对象身份区；详情路由不得残留目录 Header 或搜索控件。

### 4.5 Domain widget

领域 Widget 只有在 DOM 命中区本身表达图形几何时才能保留原生交互节点，例如 WorkGraph 的边中点、节点卡和折叠计数；该例外只允许自定义几何，不允许在原生 button 可以表达时用 `div role=button` 和手写键盘事件模拟控件。缩放、搜索、定位、关闭、保存等标准动作仍必须复用 UiButton / UiIconButton，浮动工具条和搜索面复用语义 Surface，不能因位于画布内部而复制一套 hover、focus、圆角或阴影。

Widget 可以认识 Agent、Room、Goal 等产品对象，但只组合下层合同，不重新定义基础视觉。Conversation 的 Composer 浮动工作栈属于 conversation widget，不应为了复用 DM/Room 而放进全局 `shared`。

生成式结构化问答的选项行可以由领域 pattern 保留原生 `fieldset`、radio/checkbox 与内嵌无壳 textarea，因为命中区和选择标记共同表达题目几何；拒绝、提交等标准动作仍必须使用 `UiButton`，题目、说明、提示和终态摘要仍必须选择 App Typography role。原生语义例外不是页面复制按钮或字号配方的许可。

## 5. 抽象与晋升规则

发现重复时按以下顺序判断：

| 重复事实 | 抽象位置 |
| --- | --- |
| 同一颜色、圆角、阴影、层级或关键尺寸 | Token |
| 多个视觉 class 总是共同出现 | Recipe |
| DOM、交互、键盘与 ARIA 相同 | Primitive |
| 响应式布局、浮动几何或组件组合相同 | Pattern |
| 业务对象和业务状态相同 | Entity/Feature/Widget |

默认从业务局部实现开始。第二个消费者出现时比较差异；跨两个领域出现第三个稳定消费者，或交互/可访问性必须全局一致时，再晋升到 shared。单消费者透传 wrapper、只为缩短 className 的组件和假想未来复用不得晋升。

## 6. 视觉与交互治理

### 6.1 阴影

阴影表达空间高度，不表达业务重要性：

- 普通 button、nav row、panel、card 默认无阴影；
- primary action 通过行动色表达，不由页面附加阴影；
- menu/popover、dialog 与真正悬浮的 floating action 使用对应 elevation；
- selected/current 使用中性背景、文字或位置表达，不使用阴影；
- 业务代码不得写 `shadow-[...]`；`features/pages` 的任意阴影门禁基线为零，真实 elevation 必须选择语义 recipe/token。

### 6.2 状态

`hover / active / selected / pressed / primary / focus-visible / running` 是不同语义。即使当前颜色接近，也必须使用不同 token/recipe，使后续设计可以独立调整。颜色不得成为状态的唯一信号。

### 6.3 浮层与小窗口

- anchored overlay 的 gap、viewport inset、碰撞、翻转、滚动跟随和 Portal 由共享定位层负责；
- 消费者只能选择 `placement / align / size / density / collisionPadding / layer`；
- dialog 的桌面限高、窄屏 inset、固定 header/footer 与 body scroll 由 viewport variant 负责；
- 选择器和短向导使用 `viewport="compact"`，自然高度的紧凑目录使用 `compactMax`，长表单使用 `adaptive` 或 `adaptiveMax`，图片/短文本查看使用 `visualPreview` 或 `documentPreview`，大型图形/对照工作台使用 `workbench`；内容量不是业务侧发明相近像素高度的理由；
- 不允许业务弹窗复制 `82dvh / 760px / 16px` 等产品级视口公式；
- 业务弹窗宽度只通过 `size` 选择，禁止在 Shell 上补写 `max-width / vw`；如果现有档位不合适，先判断是否真的是可跨业务复用的新内容类型；
- 单行命名/创建类 Prompt 使用紧凑决策宽度，多行输入才提升一档；Prompt 的 Header、Input 与确认动作由共享 Decision Dialog 统一，业务页面不得用局部宽高或弱化的私有按钮修补；
- z-index 只通过语义 layer 使用，禁止通过增加整数解决遮挡；嵌套 modal 的顺序由 modal stack 负责。
- 全局 feedback 固定复用 popover 材质、`feedback` 语义 layer、App Typography 与共享 Button；业务只提供已经确认的标题、影响、下一步和至多一个动作，不得通过局部阴影、圆角或 z-index 抬高反馈。

### 6.4 响应式

- 仅布局变化使用 CSS media/container query；
- 由组件自身宽度决定的布局优先 container query；
- 只有行为发生变化才使用 `useMediaQuery`；
- 产品断点通过共享语义入口使用，业务组件不得新增近似断点；
- 窄屏不建立第二套主题或第二套组件，只改变密度、排列和导航呈现。

## 7. 注释与文档合同

业务入口、状态机、协议 mapper、复杂 hook 和跨文件基础组件使用三行文件契约：

```ts
// INPUT: 接受的可信事实、上游资源或用户动作。
// OUTPUT: 对外产生的视图、命令、状态或副作用。
// POS: 在模块中的唯一职责，以及明确不负责的内容。
```

要求：

- 修改输入、输出、所有权或副作用时同步更新文件契约；
- 注释解释“为什么、边界和失败语义”，不复述函数名或 JSX；
- 导出的复杂类型/函数只在调用者无法从签名判断约束时写 TSDoc；
- 每个 entity/feature/widget 根目录最多保留一份职责文档；只有独立状态机或协议边界才增加子目录文档；
- 文档写稳定不变量，进行中的迁移计划必须标记 `non-normative`；
- 代码、测试和文档冲突时，不得只改其中一相后结束任务。

## 8. 测试合同

| 层级 | 必测内容 |
| --- | --- |
| Token/Recipe | 语义入口存在、禁止值不再新增、主题映射完整 |
| Primitive | DOM、真实键盘事件、焦点、ARIA、disabled 与状态组合 |
| Entity model | mapper、selector、状态机、recovery 与 stale response fence |
| Feature | 一次用户动作从输入到 command/result 的完整状态流 |
| Widget | 关键组合状态、窄屏结构和资源失败降级 |
| Page | 路由、恢复与少量主路径浏览器 smoke test |

源码正则只能作为架构或禁止项门禁，不能替代组件行为测试。涉及布局、Portal、碰撞和视口尺寸的 UI 必须使用真实浏览器验证。

`features/pages` 中的任意阴影与数字 z-index 已归零；`frontend-foundation-contract.test.mjs` 直接禁止两者，不再保留可上调的逐文件额度。真实 elevation 和浮层顺序只能选择语义 token、recipe 与 layer。

测试入口固定为：

- `src/**/*.test.tsx`：与 primitive/pattern 共置的 Vitest + jsdom 行为测试，必须通过 Testing Library 从角色、名称和真实用户事件观察组件；
- `scripts/*.test.mjs`：纯模型、协议、架构边界和禁止项合同；不得在这里伪造 DOM 交互结论，统一入口以有界并发运行，避免大量独立 Vite 转换进程使门禁随机崩溃；
- `npm run test:components` 与 `npm run test:contracts` 可分别定位失败，`npm test` 必须串行覆盖两类测试。

视觉回归矩阵至少覆盖：

- light / dark / rain；
- 320px、产品窄屏断点附近和桌面宽度；
- default / hover / focus / disabled / selected / loading / error；
- 中英文长文案与 reduced motion。

当前共享组件的浏览器验收入口固定为
`http://localhost:3000/ui-gallery.html?theme=light&locale=zh`。它是独立的开发
HTML 入口，不经过登录态、业务 API 或产品路由，也不得加入 Vite 的生产
`rollupOptions.input`。`theme=light|dark|rain`、`locale=zh|en` 与
`section=foundation|content|interaction|workspace|coverage` 必须写回 URL，使
人工复查和后续截图工具使用同一可复现地址。陈列面必须直接 import 并渲染
真实 `shared/ui` 组件；不得建立只为截图存在的视觉替身。公开 React 组件必须
进入唯一覆盖清单：可视组件直接渲染，复合组件的内部原语由真实父组件覆盖，
Provider 和 SVG Filter 等无独立界面的基础设施明确标注其真实消费路径。新增
公开组件但没有登记时，Gallery 覆盖合同必须失败。

组件陈列面是浏览器验证夹具，不是截图结论本身。任何影响布局、Portal、
碰撞、视口或交互状态的改动，仍须按上方矩阵在真实浏览器中检查；自动截图
基线尚未接入前，提交说明必须明确实际检查过的主题、宽度和状态，不得用
jsdom 或源码正则声称视觉通过。

## 9. Agent 修改流程

后续 Agent 修改前端时必须：

1. 先定位所有者与现有 primitive/pattern，不以页面搜索结果直接复制实现；
2. 修改公共视觉前列出受影响消费者，判断应改 token、recipe、primitive 还是 pattern；
3. 业务页面需要覆盖公共组件视觉时，先证明是新的稳定 variant，而不是添加任意 class；
4. 同步 `INPUT / OUTPUT / POS`、模块文档和唯一规范；
5. 添加与变更层级匹配的测试；
6. 运行目标测试、`npm run lint` 与 `npm run typecheck`；
7. UI 改动检查窄屏、三主题、键盘焦点和叠层关系；
8. 用户可见变化同步 `CHANGELOG.md`。

## 10. 迁移阶段（non-normative）

1. **基础门禁**：冻结新的反向依赖、任意高 z-index、重复 dialog viewport 和公共组件视觉覆盖；
2. **Primitive 收口**：Button、Form、Dialog、Overlay、Menu、Tabs 补齐语义 API 与行为测试；
3. **Pattern 收口**：统一 ResponsiveDialog、AnchoredPopover、FilterBar、SettingsSection 与 Conversation 浮动工作栈；
4. **所有权迁移**：按业务切片迁移 `hooks / store / types / lib/api`，拆分 `conversation/shared`；
5. **视觉回归**：组件陈列面已建立；继续接入可移植的浏览器截图矩阵与基线审查；
6. **清债**：移除兼容导入、闲置组件、无差异 variant 和过细目录文档，开启强制门禁。

迁移状态不得改变上文规范；尚未迁移的旧代码是已知债务，不是新代码继续复制的先例。
