# Nexus Design System

> 更新日期：2026-07-25

本文是 Nexus 前端视觉与交互规则的唯一入口，只写「判断对错所需的规则」。具体数值由 `web/src/app/styles/` 的 token 与配方代码承载；本文与代码冲突时以本文为准并修齐代码。协议、业务流程与领域状态仍由 `docs/` 下对应规范负责。

## 1. 视觉主张

Nexus 是让人、Agent 和任务持续协作的工作台：一张安静的工作纸，而不是装满卡片的控制台。

> **安静的蓝色协作工作台：暖中性画布、编辑器式留白、单一任务焦点，以及只在需要行动时出现的 Nexus 蓝。**

视觉语言由暖纸画布、克制字重与安静 chrome 构成；Nexus 蓝承担品牌行动色，京华老宋体承担对话中文正文的阅读识别。

三个不变量：

- **任务强于界面**：先读到对话、结果和下一步，而不是卡片、状态装饰或背景效果。
- **蓝色只表达行动**：主操作、当前选择、键盘焦点、链接与可继续协作的入口使用 Nexus 蓝；蓝色不是填充色。
- **协作必须可扫描**：谁在工作、进展如何、需要谁决定，靠位置、文本和图标读懂，不只靠颜色或动画。

| 维度 | 规则 |
| --- | --- |
| 构图 | 一个页面只保留一个主要工作焦点；主区域相对工作平面居中。 |
| 内容 | 页面先回答「我在哪里、谁在工作、下一步是什么」；没有操作价值的摘要、标签与说明默认删除。 |
| 交互 | hover、focus、运行、完成只做局部反馈；动效建立确认感，不抢阅读节奏。 |

适用于 Launcher、`/app` 工作台、DM / Room、工作区、设置页与 `shared/ui`。

## 2. 空间与页面

```text
canvas → rail / directory → work plane → focused surface → popover / dialog
```

- `body` 是纯色画布；常规工作面不铺全局纹理、光晕或渐变（rain 的天气效果只存在于环境层，不覆盖内容和交互）。
- 分栏之间用低对比 `1px` 分隔线与背景明度差分层；只有临时浮层、聚焦输入与真正独立任务允许上浮；panel、rail、nav row 不用常驻外阴影。
- 左侧导航只承担定位、切换与简短摘要；空状态 / 新会话只保留一个视觉锚点：欢迎信息 + Composer（或一个明确的创建动作），不得扩成模板卡片墙。
- 密度：导航、目录、设置可以紧凑；阅读、输入和工作结果必须宽松。条目共享中心线，长标题单行截断。主内容最大宽度由 `--workspace-detail-max-width`（980px）承载，不随超宽桌面继续拉长。
- 窄屏只改变密度与导航呈现（rail 收拢或 overlay），不另造一套主题；主内容相对可用 work plane 居中，不因 rail 隐藏而跳位。

页面职责：

| 页面 | 主焦点 | 禁止 |
| --- | --- | --- |
| Launcher | 一个入口动作或当前恢复会话 | 营销视觉进入 `/app` 工作面 |
| App 首页 / 新会话 | 欢迎信息 + Composer | 模板、统计、说明卡片并列竞争 |
| DM / Room | 时间线、协作状态、Composer | 侧栏式状态面板挤压正文 |
| Workspace | 文件、编辑器或预览本身 | 给每一块工具区再套卡片 |
| Settings | 当前设置组与保存动作 | 大图、渐变 banner 替代可扫描的表单 |
| Overlay | 当前选择、确认或短流程 | 关闭后仍改变底层布局或承载长期信息 |

## 3. 色彩与 Token

Token 分四层：主题基础（`--primary`、`--border`、状态色）→ 语义表面（`--surface-*`、`--modal-*`、`--button-*`、`--chip-*`）→ 交互状态（`--interaction-*`、`--text-*`、`--icon-*`、`--divider-*`）→ 组件配方（`theme-recipes.css`）。业务代码只能消费语义层与配方，不得复制 raw color、渐变和阴影。

### 3.1 主题

Nexus 蓝是唯一通用强调色，三主题共用一个语义：**`--primary` 就是行动蓝**。

| 主题 | 画布 / 文字 | `--primary` | 规则 |
| --- | --- | --- | --- |
| `light` | 暖白 `#f9f9f7` / 近黑 `#131313` | `#5b72ff` | 像纸张上的蓝色批注；画布不可偏冷蓝。 |
| `dark` | 深墨 `#131316` / 冷白 | `#8ea4ff` | 保留深色阅读对比；蓝色只提升可操作项。 |
| `rain` | 石板 `#39424d` / 冷白 | `#9ab7ff` | 环境主题，不改变信息层级。 |

规则：

- `--primary` 用于主操作、active、focus ring、链接、进度与 inline code 墨水；浅蓝背景一律用 `color-mix` 从蓝色推导，不另造浅蓝色系。实底按钮使用更深一档的 `--brand-action`。
- `--accent` 只用于文件、记忆等领域语义，不得与行动蓝争夺品牌注意力；成功、警告、危险保持语义色。
- 禁止把蓝色大面积铺到 rail、页面背景、普通卡片或所有按钮上。
- 颜色不是唯一状态信号：进行中、危险、禁用与选择态必须同时有文案、图标、边界或位置变化。

### 3.2 视觉语法

| 视觉语法 | Nexus 实现 |
| --- | --- |
| 暖白画布与纸张感中性色 | 所有 light 工作面的基础 |
| 单一品牌主操作 | `--primary` Nexus 蓝 |
| Serif 欢迎语 | 只在 Launcher / 新会话短欢迎语；CJK 继续用 Sans |
| 无底图标、低对比 hover | hover 底色用暖中性，不把图标包进彩色方块 |
| 白色 Composer、细边界、极轻阴影 | focus / 可发送状态用 Nexus 蓝 |
| 暖灰 active row | 低对比；当前协作入口可叠加极浅蓝 |

禁止使用与协作状态无关的多色主操作、装饰性星芒、订阅胶囊语义或产品功能名作为视觉元素。

## 4. 字体与排版

| 角色 | 使用 | 禁止 |
| --- | --- | --- |
| `--font-sans` | 导航、控件、元信息与对话中的拉丁正文 | 连续中文对话正文 |
| `--font-prose` | 对话中的中文正文；主字体为京华老宋体 | 导航、按钮、代码与工作区文件 |
| Launcher 品牌字（Striper / Panchang） | Launcher / 新会话的短欢迎语 | 对话正文、设置页、导航、长 CJK 标题 |
| `--font-mono` | code、kbd、源码、纯文本预览 | 连续 prose 或导航 |

字号阶梯：`10 / 11 / 12 / 13 / 15 / 17 / 22 / 28 / 42px`（Tailwind 主题键 `--text-2xs / xs / compact / sm / base / md / lg / xl / 2xl`）。对话正文是独立阅读层，遵循 §6.2；除此之外业务组件不得散落新的 `text-[Npx]`。

- 新会话欢迎：`2xl`、常规字重、宽松行高；一行主要信息 + 一个蓝色 Nexus 签名。
- 侧栏 / 设置：`--font-sans`；条目紧凑、组标题可扫描、摘要不抢正文。
- 字重使用克制阶梯：正文 `400`，表单 label `500`，标题与语义强调 `600` 封顶；不用全大写、宽字距、满屏 semibold 或彩色标题制造层级。

## 5. 表面、组件与几何

### 5.1 表面配方

| 配方 | 规则 |
| --- | --- |
| `desktop-rail` | 低对比表面 + 细分隔线；无常驻阴影 |
| `surface-panel` | 只在承载独立任务或可操作内容时单层填充 + 轻边界 |
| `surface-inset` / `surface-card` | 默认透明或低对比底；卡片必须有内容或交互职责 |
| `surface-popover` | 高不透明度、清晰边界、单档阴影 |
| `input-shell` | 背景、边界与轻 ring 建立焦点；不用玻璃或发光描边 |
| `composer-shell` | 主动作表面；常规页面唯一允许明显上浮的白色 surface |

### 5.2 圆角与阴影

圆角层级表达对象职责，组件不得自行挑选 `rounded-*`：

```css
4px   micro        /* kbd、极小承载体 */
6px   control-xs   /* 紧凑图标、列表当前态 */
8px   control      /* 默认按钮、menu item、input、方形头像 */
10px  control-lg   /* 大输入、选择器、目录头像 */
12px  content      /* 独立可操作内容面、user 消息壳 */
16px  overlay      /* popover、dialog、sheet */
20px  composer     /* 唯一的主输入焦点 */
24px  shell        /* 认证、故障等独立壳 */
999px full         /* 状态点、真实 pill、明确要求的圆形头像 */
```

- button、input、menu item 用 `control`；不用 `lg`、`xl` 或 pill 制造「高级感」，不把常规按钮和筛选项 pill 化。
- 聊天 / 目录的人物头像是方形，圆角取 control 系（8–12px 随尺寸）；禁止 `full` 使头像近似圆形。
- 常规 panel、rail、nav row 不使用外阴影；Composer 只用极轻边界与低透明阴影让白色 surface 脱离画布；popover / dialog 只一档更清晰的阴影，不用内阴影、双描边、发光或毛玻璃。
- 不新增无消费者的渐变；删除后信息层级不变的渐变默认删除。

### 5.3 按钮与图标操作

原则：默认退后，行动才出现。一个工作上下文只允许一个视觉上明确的 primary action，其余用 tonal 或 ghost 降级。

| 类型 | 几何与表面 | 场景 | 禁止 |
| --- | --- | --- | --- |
| `primary` | 32–36px 高、`control` 圆角、`--brand-action` 实底、白字 | 发送、保存、创建、确认等唯一主动作 | 同一工具栏两个以上蓝按钮；渐变或厚阴影 |
| `secondary` / tonal | 32px、暖灰轻底、深字、默认无阴影 | 次级确认、筛选入口、低风险动作 | 蓝色描边假装 primary |
| `ghost` | 28–32px、透明底、hover 才出暖灰底 | toolbar、导航、轻操作 | 常驻边框或大面积背景 |
| `destructive` | 同层级几何、语义红 | 删除等不可恢复操作 | 品牌蓝；只靠红色区分危险 |
| `icon` | 32px 方形命中区、图标 18–20px、无底无框默认 | 搜索、菜单、附件、关闭、编辑 | 圆形蓝底图标泛滥；图标与文字竞争 |
| `icon compact` | 28px 命中区；只用于紧凑列表或 code chrome | 行内次级操作 | 移动端主操作 |

状态统一：default 可读；hover 只轻微改背景 / 边界 / 文字明度；pressed 局部色深不位移整块布局；focus 2px Nexus 蓝 ring 并留隔离；disabled 降对比不隐藏、保留可解释文案；loading 保持原尺寸与文案占位。

- primary hover 只加深蓝，不换色相；secondary / ghost hover 用暖中性底。
- label 用常规到 medium 字重；不用 `font-bold` 弥补层级不足。
- 文本按钮水平 padding 12–14px；icon 与 label 间距 8px；触屏命中区至少 36px。
- 「保存」「发送」「继续」等主动作只在可执行时变蓝；无输入或无变更时保持中性 disabled。

### 5.4 输入、选择器、菜单与提示

- text input / select 默认 36px 高、`control` / `control-lg` 圆角、白色或轻暖灰底、`1px` 控制边界；focus 只强化蓝色 border / ring，不加高饱和蓝底或 glow；placeholder 用 `--text-muted` 且必须可读。
- segmented control 是选择器不是导航标签墙；整体轻底，active 用白色或极浅蓝 surface，边界与阴影极轻。
- switch / checkbox / radio checked 时用 Nexus 蓝；label 与描述承担解释，颜色只确认状态。
- menu / popover：`overlay` 圆角、白色高不透明度、`1px` 边界、单档阴影；条目 `control` 圆角、hover 暖灰。
- dialog 只承载确认、编辑或短流程；标题、内容、footer 不重复套 card；危险操作保留明确文案与 destructive action。
- tooltip 紧凑单行，只解释无文字的图标，不替代按钮 label。
- chip / pill 只用于真实筛选、状态或可移除实体；没有交互或信息密度价值时不用。

### 5.5 高优先级组件规则

**导航与目录**

- rail 只表达位置、切换、未读和简短运行态；当前项用中性浅灰底（可叠加极浅蓝），不靠粗体、大色块或填充图标。
- icon 统一 18–20px、单色、细描边、与文字同色；不建独立的彩色 icon 系统。
- 最近会话、文件或联系人保持纯文本列表；长内容截断，不额外添加摘要卡。
- Nexus 主智能体与普通 DM 共用同一目录行，只保留“固定置顶、不可删除”两项产品差异；不得另建一级导航动作、专用 Focus 侧栏或第二套激活状态。

**顶部 tab 与 header 工具**

- tab 托盘只用极轻底与细分隔线；active tab 用白色 surface，inactive 之间仅用 hairline 分隔。
- view 切换、历史等 header 工具默认无容器：hover 出暖灰底，active 用暖灰或极浅蓝底，不叠加边框囊与阴影。

**Composer**

- Composer 是一个主任务面，不是一组按钮的容器。输入区上部只归文字；底部工具行左端归附件，右端归模式、状态与发送；次级能力默认只显示图标或短标签，不常驻快捷键说明。
- 发送按钮位于底部工具行右端：默认是 ghost 态，仅在可发送、运行或需要停止时显式呈现状态色。
- Composer 列比正文列略宽（正文 800px → Composer 880px），仍是常规页面唯一明显上浮的白色 surface。
- runtime 等弱标注放在壳外下方居中的说明行，不占壳内工具行。
- focus 用 Nexus 蓝 ring；不用蓝底填满整个输入壳。

**对话与协作状态**

- 时间线负责顺序，正文、最终状态和摘要留在 Feed；过程细节进入 Thread。
- guide、queue、wake、Todo 与权限请求贴近目标内容，不伪装成完整聊天气泡。
- Agent 正在工作用文字、状态点、进度或位置表达；蓝色仅强调当前可操作项。

**工作区与设置**

- 文件标题、路径、编辑器工具栏和预览 chrome 由共享容器负责，内容渲染器不重复实现工具栏。
- 代码、表格和预览保持工具界面，不套厚卡片。
- Settings 用分组、分隔线和表单密度表达层级；不做 banner、渐变 hero 或卡片瀑布流。

## 6. 内容渲染

核心判断：**答案首先是一篇可读的编辑文本；只有可执行、可展开或必须比较的数据才成为 surface。** Assistant 正文不是聊天卡，应直接落在工作平面上；用户消息、工具过程、权限问题和 Artifact 只有在承担明确角色时才上浮。

### 6.1 内容层级

| 内容 | 视觉职责 | 是否上浮 |
| --- | --- | --- |
| Assistant 正文 | 主要阅读层；直接在 canvas 上排版 | 否 |
| User message | 明确提问边界；紧凑中性消息面 | 仅必要的轻 surface |
| 标题、段落、列表、链接 | 组织阅读顺序 | 否 |
| quote、inline code | 文内语义强调 | 否，只用低对比局部底色 / 边界 |
| code block、table、image | 需要独立阅读或横向滚动 | 是，低对比工具面 |
| tool / thinking summary | 过程状态；默认折叠 | 仅 header 或展开内容 |
| AskUserQuestion / permission | 要求用户决定的交互 | 是，唯一明确的局部 task surface |
| Artifact | 可打开、预览或下载的产物 | 是，紧凑的可操作行或预览面 |
| error / warning | 需要识别和恢复的状态 | 是，但只使用语义色和短文案 |

### 6.2 Markdown 与 prose

`nexus-chat-markdown` 是对话正文的唯一排版入口，`nexus-workspace-file-markdown` 是工作区文件的唯一排版入口；两者共享语义语法，但保持各自的阅读密度。

- 对话正文固定 `16px / 1.65rem / 400`：中文片段使用 `--font-prose`（京华老宋体），拉丁文字使用 `--font-sans`，术语与 code 使用 `--font-mono`。
- 正文直接子块固定 `12px` 栅格；普通段落不再叠加独立 margin，避免博客式大留白。
- Assistant prose 默认无背景、无边框、无 bubble、无阴影；强调用 `font-semibold`（600）与文字色，不用蓝色或彩色底大面积强调；`em` 只表达语义强调。
- 标题：`h1 22px / h2 18px / h3–h6 16px`，均 `600`；上方 12px（h3+ 为 8px）、下方收紧 4px。不用 display serif、彩色标题、全大写或过度 letter-spacing。
- `hr` 只分隔真正不同的主题；半像素细分隔线，不作为每段装饰。
- 列表用浏览器原生 `disc` / `decimal` marker，缩进与条目间距克制；有序列表用 tabular number，marker 用 `--text-default`，不是品牌蓝；嵌套列表只增加缩进，不形成层层 card。
- 链接用 Nexus 蓝文字，默认可辨识，hover / focus 再出下划线或局部底色；外部链接、workspace 文件、Agent mention 保留各自语义图标或标签。
- quote 用中性左边界与次级文字色，背景透明；不用大面积蓝底或强制 italic。
- Workspace 文件保持紧凑 `14px / 1.55`，标题按 `20 / 18 / 16px` 递减；不变成第二套博客主题。
- 流式文本按完整块稳定落位；不因 token 到达反复跳动、改写已稳定段落或挤动阅读位置。

### 6.3 Code、表格、媒体与 Artifact

- inline code 是正文中的术语，不是动作链接：`--font-mono`、轻中性背景、细边界和统一的 Nexus 蓝墨水；可点击性仍由链接样式与 hover 反馈表达。kbd 共用紧凑几何，可保留极轻内侧底边表达实体按键。
- code block 是低对比工具面，不是高饱和暗色「开发者卡片」：近白 / 暖灰表面、细边界、`content` 圆角；语言标签固定在左上，复制等必要操作只在 hover / focus 浮到右上，不做厚重工具栏；流式 fence 闭合前保持稳定 shell。
- table 只在比较、参数、结果矩阵或结构数据需要时使用：透明底、无外层圆角卡片、细分隔线、铺满阅读列；表头用 `600` 字重不全大写；不做 zebra stripe、厚网格或浮雕卡片；窄屏由外层负责受控横向滚动。
- 图片按自然比例展示，圆角 + 细边界，不套卡；caption 用 12px 次级文字紧贴媒体；图片操作 hover 才显现。
- Mermaid、图表和 Office 预览是独立懒加载内容面：加载态保留确定尺寸，错误态展示可恢复说明与动作，不使用抽象装饰插画。
- Artifact 是产物入口，不是聊天气泡：单行或两行紧凑操作行（类型图标、名称、路径摘要、打开动作）；预览只在内容本身有阅读价值时展开；文件集合保持列表，不变成彩色卡片网格。

### 6.4 工具过程、问答、权限与错误

- 工具调用、搜索、读写和思考摘要默认单行过程项：状态图标、简短动词、目标摘要和可展开 affordance；running 用局部 spinner 或文本状态；completed 收敛为低对比结果行；不绘制大面积蓝色进度卡。
- thinking 只显示产品允许公开的摘要；展开行为不得改变已读正文的因果顺序。
- AskUserQuestion / permission 是用户必须行动的唯一强交互 surface：`content` 圆角、单层边界、清楚问题、最少必要选项和一个蓝色提交动作；选项是行级选择器，不是彩色答案卡；提交后收敛成一条完成状态或可展开摘要。
- 权限解释必须说明将执行什么、作用域和风险；拒绝与取消是 ghost / secondary，不把所有动作做成 primary。
- error / warning 短、具体、可恢复：问题、影响、下一步动作；语义色只服务图标和关键文本，不铺满整块背景。
- empty state 只保留一个下一步动作和必要说明；不使用插画、统计卡或大段教学文案填空。

### 6.5 对话身份与消息表面

- User message 用紧凑、暖中性的低对比 surface 与 Assistant 正文区分：无 Nexus 蓝泡泡、无边框、无阴影、`content` 圆角；不重复显示用户头像或昵称。时间、重跑、编辑与复制贴在消息下方；Desktop 仅在 hover / focus 显现，窄屏保持可达。
- Assistant / Agent 输出默认无 bubble。展开态身份行只保留 32px、`control` 圆角头像与垂直居中的名称；正文与身份行间距 8px，时间和模型不抢正文第一行。
- 底部统计（时长、Token、费用、缓存、模型）与复制动作始终共享同一行；窄空间依次截断时长、Token、费用与缓存，模型英文名和复制动作保持完整且不换行。
- Room 中不同 Agent 的可区分性优先来自名称、头像、顺序与状态；不为每个 Agent 分配独立品牌色。
- system、guide、queue、wake 等控制信息是辅助行，不作为普通发言者，也不与最终答案竞争。
- 轮次级操作（复制、重试、展开）默认 hover / focus 露出，键盘可达；不在每段正文旁常驻一排图标。

### 6.6 响应式与动效

- `620px` 以下收紧消息壳、标题和列表列宽；优先保留正文宽度、点击区域、滚动位置和 Composer。
- 列表 marker 始终占独立列，不用负 margin 压进正文。
- hover / focus 用 `--motion-duration-fast: 160ms`；面板与 overlay 用 `--motion-duration-normal: 220ms`；统一 `--motion-ease-standard`。
- 内容流只允许三类动效：新轮次的微弱进入、过程项状态更新、展开 / 收起 detail；正文不做逐字弹跳或大范围 stagger。
- `:focus-visible` 必须保留隔离线与 `--ring`；`prefers-reduced-motion` 下动画近乎即时。

## 7. 实现入口与检查

| 责任 | 文件 |
| --- | --- |
| 主题与语义 token | `web/src/app/styles/theme-tokens.css` |
| 字体、焦点、滚动条、减少动效 | `web/src/app/styles/theme-base.css` |
| surface、dialog、input、Markdown、响应式配方 | `web/src/app/styles/theme-recipes.css` |
| 样式入口与主题变体 | `web/src/app/globals.css` |
| Markdown、代码壳、流式与工作区产物 | `web/src/shared/ui/markdown/` |
| 对话内容块、工具过程、问答与 Artifact | `web/src/features/conversation/shared/message/blocks/` |
| 工作区内容与预览 | `web/src/features/conversation/shared/editor/` |

业务组件只能消费 token 与共享配方，不定义第二套视觉规则；新增或移动视觉基础设施时，同步更新对应目录的 `AGENTS.md`。

提交视觉改动前确认：

1. 页面能否按「canvas → rail → 工作平面 → 聚焦 surface → 浮层」读懂，且只有一个主要工作焦点？
2. 蓝色是否只服务行动、选择和焦点，而非背景装饰？
3. button、icon button、input、menu、dialog、chip 是否都使用既定高度、圆角与状态配方？
4. Assistant 正文是否无 bubble、无卡片即可读；code、table、tool、question、Artifact 是否只在承担独立职责时上浮？
5. 是否使用语义 token，而非复制 raw color、渐变、阴影或 `rounded-*` 近似值？
6. 三主题、窄屏、键盘焦点、disabled、loading、streaming、destructive 与减少动效是否仍可用？

如果删除一个渐变、圆角、标签、卡片或动画后，用户仍能更快判断位置、状态和下一步，默认删除它。
