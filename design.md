# Nexus Design System

> 更新日期：2026-07-24

本文是 Nexus 前端视觉与交互规则的唯一入口。协议、业务流程与领域状态仍由 `docs/` 下对应规范负责。

## 1. 视觉主张

Nexus 是一个让人、Agent 和任务持续协作的工作台。它应该像一张安静的工作纸，而不是装满卡片的控制台。

> **安静的蓝色协作工作台：暖中性画布、编辑器式留白、单一任务焦点，以及只在需要行动时出现的 Nexus 蓝。**

Nexus 的组件语言以留白优先、低装饰导航、单一 Composer 焦点、轻边界、克制阴影、紧凑按钮、真实的圆角层级与局部反馈为基础；所有品牌强调统一使用 Nexus 蓝。

### 三个不变量

- **任务强于界面**：用户应先读到对话、结果和下一步，而不是卡片、状态装饰或背景效果。
- **蓝色只表达 Nexus 的行动**：主操作、当前选择、键盘焦点和可继续协作的入口使用蓝色；蓝色不是全页填充色。
- **协作必须可扫描**：谁在工作、任务进展和需要谁决定，必须通过位置、文本和图标读懂，不能只靠颜色或动画。

### 构图、内容与交互的共同约束

| 维度 | 规则 |
| --- | --- |
| 构图 | 一个页面只保留一个主要工作焦点；主区域相对工作平面居中，而不是相对整个浏览器随意居中。 |
| 内容 | 页面先回答“我在哪里、谁在工作、下一步是什么”；没有操作价值的摘要、标签与说明默认删除。 |
| 交互 | hover、focus、输入、运行和完成只做局部反馈；动效建立确认感，不能抢走阅读节奏。 |

适用于 Launcher、`/app` 工作台、桌面导航、DM / Room、工作区、设置页与 `shared/ui`。

## 2. 空间与页面

```text
app canvas
  ├─ navigation rail / directory
  └─ work plane
      ├─ context header（仅在确有定位或动作需要时）
      ├─ conversation / workspace content
      ├─ composer or primary action surface
      └─ popover / dialog / temporary overlay
```

### 2.1 工作平面

- `body` 是平静的纯色画布；常规工作面不铺全局纹理、光晕或渐变。
- 左侧导航只承担定位、切换和简短摘要；主工作平面承担阅读、编辑与协作。
- 主区域的空状态或新会话只保留一个视觉锚点：欢迎信息与 Composer，或一个明确的创建动作。不得把它扩成模板卡片墙。
- 分栏之间优先使用低对比 `1px` 分隔线。只有临时浮层、聚焦输入与真正独立任务才允许上浮。
- Desktop rail、directory、header 与 footer 使用背景明度差和细分隔线分层，不使用常驻外阴影。

### 2.2 页面职责

| 页面 | 主焦点 | 禁止 |
| --- | --- | --- |
| Launcher | 一个入口动作或当前恢复会话 | 把营销视觉带进 `/app` 工作面 |
| App 首页 / 新会话 | 欢迎信息 + Composer | 模板、统计、说明卡片并列竞争 |
| DM / Room | 时间线、当前协作状态、Composer | 侧栏式状态面板挤压正文 |
| Workspace | 文件、编辑器或预览本身 | 给每一块工具区再套卡片 |
| Settings | 当前设置组与保存动作 | 用大图、渐变 banner 替代可扫描的表单 |
| Overlay | 当前选择、确认或短流程 | 关闭后仍改变底层布局或承载长期信息 |

### 2.3 密度

- 导航、目录和设置可以紧凑；阅读、输入和工作结果必须宽松。
- 导航条目、头像、标题与摘要共享中心线；长标题单行截断，不把侧栏扩成数据面板。
- 主内容的最大宽度只服务于可读性：`--workspace-detail-max-width: 980px` 保持不变；Nexus Composer 固定上限为 `800px`，纯正文阅读列以 `768px` 为舒适基准。Nexus 对话 Feed 使用 `844px` 外框，消息 frame 为 `820px`。Assistant 顶部身份行只保留头像和与其垂直居中的名称；正文和统计跨满 frame，左边缘对齐 Assistant 头像外边缘，时间与模型不混入展开态身份行。User 的身份行和内容壳都位于头像左侧，与头像保持固定 `12px` 间距，头像仍是该消息 frame 的最右端。不得随超宽桌面继续拉长。
- 窄屏只改变密度与导航呈现方式，不另造一套主题。

## 3. 色彩与 Token

Token 保持四层映射。业务 JSX 和业务 CSS 只能消费语义层或组件配方，不得复制 raw color、渐变和阴影。

1. **主题基础**：`background`、`foreground`、`primary`、`border`、`ring`、状态色。
2. **语义表面**：`surface-*`、`modal-*`、`input-shell-*`、`button-*`、`chip-*`。
3. **交互状态**：`interaction-*`、`text-*`、`icon-*`、`divider-*`。
4. **组件配方**：`desktop-rail`、`surface-panel`、`dialog-shell`、`input-shell`、`composer-shell`。

### 3.1 Nexus 蓝色主题

Nexus 的品牌蓝是唯一通用强调色。当前 light token 的 `--primary: #5b72ff` 是设计锚点；后续主题调整围绕它建立明度与对比，不新增第二个品牌主色。

| 主题 | 画布 / 文字 | Nexus 蓝 | 规则 |
| --- | --- | --- | --- |
| `light` | 暖白 `#f9f9f7` / 近黑 `#131313` | `#5b72ff` | 像纸张上的蓝色批注；画布不可偏冷蓝。 |
| `dark` | 深墨 `#131316` / 冷白 | `#8ea4ff` | 保留深色阅读对比；蓝色只提升可操作项。 |
| `rain` | 石板 `#39424d` / 冷白 | `#9ab7ff` | 是环境主题，不应改变信息层级。 |

Light theme 的目标中性色：

```css
--nexus-canvas: #f9f9f7;
--nexus-rail: #f6f6f3;
--nexus-surface: #ffffff;
--nexus-surface-subtle: #f1f1ee;
--nexus-text-strong: #131313;
--nexus-text-default: #4f4e4a;
--nexus-text-muted: #77756f;
--nexus-border-subtle: #e7e7e2;
--nexus-border-control: #d8d8d2;
--nexus-primary: #5b72ff;
```

规则：

- `--primary` 用于主要动作、active、focus ring、链接和进度；轻背景一律从蓝色 `color-mix` 推导，不另造浅蓝色系。
- `--accent` 若保留，只能是协作或领域语义，不得与 `--primary` 争夺品牌注意力；成功、警告、危险保持语义色。
- 禁止把蓝色大面积铺到 rail、页面背景、普通卡片或所有按钮上。
- 颜色不是唯一状态信号：进行中、危险、禁用与选择态必须同时有文案、图标、边界或位置变化。
- `--surface-body-background` 在常规工作面解析为纯色；rain 的雾、雨滴和水花只能存在于环境层，不能覆盖内容和交互。

### 3.2 Nexus 视觉语法

Nexus 的视觉语法以暖中性画布、单一行动色和低噪音操作为核心：

| 视觉语法 | Nexus 实现 |
| --- | --- |
| 暖白画布与纸张感中性色 | 保留，作为所有 light 工作面的基础。 |
| 单一品牌主操作 | 使用 `--primary` Nexus 蓝。 |
| Serif 欢迎语 | 只在 Launcher 或新会话短欢迎语中按字体可用性采用；CJK 继续使用 Sans。 |
| 无底图标、低对比 hover | 保留；hover 底色使用暖中性，不把每个图标包进蓝色方块。 |
| 白色 Composer、细边界、极轻阴影 | 保留；focus / 可发送状态改为 Nexus 蓝。 |
| 轻边框主按钮 | 保留几何和反馈；填充、focus、active 只用 Nexus 蓝。 |
| 暖灰 active row | 保留低对比逻辑；当前协作入口可叠加极浅蓝。 |

禁止使用与 Nexus 协作状态无关的多色主操作、装饰性星芒、订阅胶囊语义或产品功能名作为视觉元素。

## 4. 字体与排版

### 4.1 字体角色

| 角色 | 使用 | 禁止 |
| --- | --- | --- |
| `--font-sans` | 导航、按钮、表单、侧栏、摘要、设置、全部 CJK 正文 | 依赖未声明的网络字体 |
| `--font-prose` | Assistant Markdown 的 Latin prose；CJK 由分段规则切回本地宋体链 | 导航、表单、侧栏或整段混合语言 UI |
| `--font-display` | Launcher 或新会话的短欢迎语；仅在排版确有价值时使用 | 对话正文、设置页、导航、长 CJK 标题 |
| `.message-cjk-text` | 聊天和 Markdown 的 CJK 片段 | 应用于整段英文或非内容 UI |
| `--font-mono` | code、kbd、源码、纯文本预览 | 连续 prose 或导航 |

`--font-display` 的作用是给稀疏的入口界面一点编辑感，不能变成品牌装饰。没有可靠的 CJK display 字体时，CJK 欢迎语继续使用 `--font-sans`，以尺寸、留白和常规字重建立层级。

字体链以 `theme-base.css` 为实现真相源。`.message-cjk-font` 只设置父级比例字体，`remarkMixedScript` 负责 CJK / Latin 分段。

### 4.2 类型阶梯与密度

界面字号阶梯保持：`2xs 10px`、`xs 11px`、`sm 13px`、`base 15px`、`md 17px`、`lg 22px`、`xl 28px`、`2xl 42px`。对话正文是独立的阅读层，固定遵循 6.2 的 `16 / 18 / 22px` 阶梯；除此之外不得在业务组件散落新的 `text-[Npx]`。

| 内容 | 规则 |
| --- | --- |
| 新会话欢迎 | `2xl`、常规字重、宽松行高；只允许一行主要信息与一个蓝色 Nexus 签名。 |
| 对话 Markdown | 16px / 1.65rem；标题 22 / 18 / 16px，均为 `700`；直接子块固定 12px 栅格。 |
| 工作区 Markdown | 14px / 1.55；列表列宽约 `1.65rem`。 |
| 源码 / 纯文本 | 13px / 1.6；保持缩进、行宽和光标可追踪性。 |
| 侧栏 / 设置 | `--font-sans`；条目紧凑、组标题可扫描、摘要不抢正文。 |

普通文字与强调文字共用深色基准，正文为 `400`，标题与语义强调为 `700`。不要用全大写、宽字距、满屏 semibold 或彩色标题制造层级。

## 5. 表面、组件与几何

### 5.1 表面层级

```text
canvas
  └─ rail / directory
      └─ work plane
          └─ focused input or task surface
              └─ popover / dialog
```

| 配方 | 规则 |
| --- | --- |
| `desktop-rail` | `--nexus-rail` 或同级低对比表面；细分隔线；无常驻阴影。 |
| `surface-panel` | 只在面板承载独立任务或可操作内容时使用单层填充和轻边界。 |
| `surface-inset` / `surface-card` | 默认透明或低对比底；卡片必须有内容或交互职责。 |
| `surface-popover` | 高不透明度、清晰边界、必要的单层阴影。 |
| `input-shell` | 背景、边界与轻 ring 建立焦点；不使用玻璃或发光描边。 |
| `composer-shell` | 新会话与会话输入的主动作表面；是常规页面唯一允许明显上浮的白色 surface。 |

### 5.2 圆角与阴影

圆角不意味着“所有东西都圆”，而是用少量几何层级表达对象的职责。Nexus 必须使用同一套层级，不能让每个组件自行挑选 `rounded-*`。

```css
--radius-micro: 4px;          /* kbd、极小承载体 */
--radius-control-xs: 6px;    /* 紧凑图标或列表当前态 */
--radius-control: 8px;       /* 默认按钮、menu item、input */
--radius-control-lg: 10px;   /* 高度较大的输入或选择器 */
--radius-content: 12px;      /* 独立可操作内容面 */
--radius-overlay: 16px;      /* popover、dialog、sheet */
--radius-composer: 18px;     /* 唯一的主输入焦点 */
--radius-shell: 24px;        /* 认证、故障等独立壳 */
--radius-full: 999px;        /* 状态点、真实 pill、明确要求的圆形头像 */
```

| 对象 | 必须使用 | 禁止 |
| --- | --- | --- |
| 小图标 hover、kbd | `micro` / `control-xs` | 给普通图标加永久方形底。 |
| button、input、menu item | `control` | 随意使用 `lg`、`xl` 或 pill。 |
| 选择器、大输入 | `control-lg` | 通过超大圆角制造“高级感”。 |
| 工具面、card | `content` | 为没有交互职责的 section 套卡。 |
| popover、dialog | `overlay` | 把 dialog 做成圆角 24px 的漂浮大卡。 |
| Composer | `composer` | 将 18px 下放到普通 panel。 |
| 聊天 / 目录的方形人物头像 | `control`（8px） | 在小尺寸上使用 content / full，使头像近似圆形。 |
| 状态点、真正胶囊、明确要求的圆形头像 | `full` | 把常规按钮和筛选项全 pill 化。 |

- 常规 panel、rail 和 nav row 不使用外阴影。
- Composer 使用极轻的 `1px` 控制边界与两层低透明阴影，目的只是让白色 surface 脱离暖白画布。
- popover / dialog 只使用一档更清晰的阴影；不使用内阴影、双描边、发光或毛玻璃。
- Dialog 复用 modal / input 语义，不能因进入浮层就变成厚玻璃卡。
- 不新增无消费者的 ambient / material 渐变。现有渐变若删除后信息层级不变，默认删除。

### 5.3 按钮与图标操作

所有按钮遵循“默认退后，行动才出现”的原则。一个工作上下文只允许一个视觉上明确的 primary action；其余动作用 tonal 或 ghost 降级。

| 类型 | 几何与表面 | 使用场景 | 禁止 |
| --- | --- | --- | --- |
| `primary` | `32–36px` 高、`control` 圆角、Nexus 蓝实底、白字、`1px` 同色边界 | 发送、保存、创建、确认等当前上下文唯一主动作 | 同一工具栏放两个以上 blue button；渐变或厚阴影。 |
| `secondary` / tonal | `32px` 高、暖灰轻底、深色文字、默认无阴影 | 次级确认、筛选入口、低风险动作 | 用蓝色描边假装 primary。 |
| `ghost` | `28–32px` 高、透明底、hover 才出现暖灰底 | toolbar、导航、轻操作 | 常驻边框或大面积背景。 |
| `destructive` | 与对应层级同一几何；只使用语义红 | 删除、撤销不可恢复操作 | 用品牌蓝或只靠红色区分危险。 |
| `icon` | 方形命中区 `32px`；图标 `18–20px`；无底默认 | 搜索、菜单、附件、关闭、编辑 | 圆形蓝底图标泛滥；图标与文字竞争。 |
| `icon compact` | 命中区 `28px`；只用于紧凑列表或 code chrome | 行内次级操作 | 用于移动端主操作。 |

按钮状态必须统一：

```text
default  → 语义表面与文字可读
hover    → 仅轻微改变背景 / 边界 / 文字明度
pressed  → 局部色深或 1px 内缩感，不位移整块布局
focus    → 2px Nexus 蓝 ring，ring 与元素边界留出隔离
disabled → 降低对比和交互，不隐藏按钮、不移除可解释文案
loading  → 保持原尺寸与文案占位，只替换局部图标或显示进度
```

实现规则：

- primary hover 只加深 Nexus 蓝，不改变为第二种色相；secondary / ghost hover 使用 `--nexus-surface-subtle`。
- primary、secondary、ghost 的 label 全部使用常规到 medium 字重；不要用 `font-bold` 弥补层级不足。
- 文本按钮的水平 padding 为 `12–14px`；icon 与 label 间距为 `8px`；不要制造过宽 pill。
- 所有可点击对象在触屏环境至少提供 `36px` 命中区；视觉尺寸可以更紧凑，但命中区不能变小。
- “保存”“发送”“继续”等语义主动作只有在可执行时变蓝；无输入或无变更时保留中性 disabled 状态。

### 5.4 输入、选择器、菜单与提示

**输入与 Composer**

- 常规 text input / select 默认高度 `36px`，使用 `control` 或 `control-lg` 圆角、白色或轻暖灰底、`1px` 控制边界。
- focus 只强化蓝色 border / ring，不能给整个字段加高饱和蓝底或 glow。
- placeholder 使用 `--text-muted`，但必须可读；不能用极低透明度制造“轻”。
- Composer 是唯一可采用 `composer` 圆角、显著留白和极轻阴影的输入面。输入区上部归文字，下部归附件、模式与动作。
- 工具控件默认无容器；hover / focus 才出现 `control-xs` / `control` 的轻底。附件在左，模式与发送等动作在右。

**select、segmented control 与 switch**

- select 维持输入壳语法，当前选项用文字和 chevron 表达，不用彩色大胶囊。
- segmented control 是选择器，不是导航标签墙；整体使用轻底，active 用白色或极浅蓝 surface，边界与阴影都极轻。
- switch、checkbox、radio 在 checked 时使用 Nexus 蓝；label 与描述承担主要解释，颜色仅确认状态。

**menu、popover、dialog 与 tooltip**

- menu / popover：`overlay` 圆角、白色高不透明度、`1px` 边界、单档阴影；条目 `control` 圆角并在 hover 变为暖灰。
- dialog：只承载确认、编辑或短流程；标题、内容、footer 不重复套 card。危险操作必须保留明确文案和 destructive action。
- tooltip：紧凑、单行、深色或高对比表面；只解释无文字的图标，不替代按钮 label。
- 单个页面的浮层不应同时出现多层 menu、dialog、tooltip；优先关闭或收纳上层状态。

**chip、badge 与状态点**

- chip 只用于真实筛选、状态或可移除实体；没有交互或信息密度价值时不用 chip。
- badge 使用文本与语义色，不作为装饰性的彩色小块。
- pill 只用于真实的连续状态、短套餐 / 权限状态或可关闭标签；不能把每个操作变成 pill soup。

### 5.5 高优先级组件规则

**导航与目录**

- rail 只表达位置、切换、未读和简短运行态；当前项使用微弱蓝色背景或边界，不靠粗体和大色块。
- icon 统一 `18–20px`、单色、细描边；与文字同色，不建独立的彩色 icon 系统。
- 最近会话、文件或联系人保持纯文本列表；长内容截断，不额外添加摘要卡。

**Composer**

- Composer 是对话页的一个主任务面，而不是一组按钮的容器。
- 输入区上部保持可读留白，底部工具按照“附件在左、模式/动作在右”分组；次级能力默认只显示图标或短标签。
- 默认状态不显示重型发送按钮；仅在可发送、运行或需要停止时显式呈现状态。
- focus 使用 Nexus 蓝 ring；不要使用蓝色背景填满整个输入壳。

**对话与协作状态**

- 时间线负责顺序，正文、最终状态和摘要留在 Feed；过程细节进入 Thread。
- guide、queue、wake、Todo 与权限请求贴近目标内容，不伪装成完整聊天气泡。
- Agent 正在工作用文字、状态点、进度或位置表达；蓝色仅强调当前可操作项。

**工作区与设置**

- 文件标题、路径、编辑器工具栏和预览 chrome 由共享容器负责，内容渲染器不重复实现工具栏。
- 工作区默认使用 `md` / `sm` 圆角；代码、表格和预览保持工具界面，不套厚卡片。
- Settings 使用分组、分隔线和表单密度表达层级；不做 banner、渐变 hero 或卡片瀑布流。

## 6. 内容渲染、响应式与动效

内容渲染遵循同一个核心判断：**答案首先是一篇可读的编辑文本；只有可执行、可展开或必须比较的数据才成为 surface。**

Assistant 正文不是一张聊天卡。它应直接落在工作平面上，由排版、顺序和少量语义块建立层级；用户消息、工具过程、权限问题和 Artifact 只有在承担明确角色时才上浮。

### 6.1 内容层级

```text
conversation round
  ├─ identity / time / local status（轻元信息）
  ├─ prose（无卡片的主要阅读层）
  ├─ structured content（code / table / quote / image）
  ├─ executable detail（tool / permission / question / artifact）
  └─ completion / actions（贴近结果的局部操作）
```

| 内容 | 视觉职责 | 是否上浮 |
| --- | --- | --- |
| Assistant 正文 | 主要阅读层；直接在 canvas 上排版 | 否 |
| User message | 明确提问边界；紧凑中性消息面 | 仅必要的轻 surface |
| 标题、段落、列表、链接 | 组织阅读顺序 | 否 |
| quote、inline code | 文内语义强调 | 否，只使用低对比局部底色 / 边界 |
| code block、table、image | 需要独立阅读或横向滚动 | 是，低对比工具面 |
| tool / thinking summary | 过程状态；默认折叠 | 仅 header 或展开内容 |
| AskUserQuestion / permission | 要求用户决定的交互 | 是，唯一明确的局部 task surface |
| Artifact | 可打开、预览或下载的产物 | 是，紧凑的可操作行或预览面 |
| error / warning | 需要识别和恢复的状态 | 是，但只使用语义色和短文案 |

### 6.2 Markdown 与 prose

`nexus-chat-markdown` 是对话正文的唯一排版入口，`nexus-workspace-file-markdown` 是工作区文件的唯一排版入口。两者共享语义语法，但不能因为共用组件而失去各自的阅读密度。

**正文**

- 对话正文固定为 `16px / 1.65rem / 400`，英文片段使用 `--font-prose`，CJK 片段继续由 `message-cjk-text` 映射到本地宋体链；技术术语与 code 仍使用 `--font-mono`。
- 正文直接子块使用固定 `12px` 栅格；普通段落本身不再叠加独立 margin，避免连续段落被写成博客式大留白。
- Assistant prose 默认无背景、无边框、无 bubble、无阴影；内容本身是视觉主体。
- 强调使用 `font-bold` 与文字色，不用蓝色或彩色底大面积强调。
- `em` 只表达语义强调；不要把整段系统提示或状态文本设为斜体。
- 流式文本按完整块稳定落位；不得因为 token 到达而反复跳动、改写已稳定段落或挤动阅读位置。

**标题与分隔**

| 元素 | 对话排版 | 规则 |
| --- | --- | --- |
| `h1` | `22px / 26.4px / 700` | 只用于答案内真正的一级章节；上方 `12px`，下方收紧 `4px`。 |
| `h2` | `18px / 26.4px / 700` | 最常用的结构标题；使用和 `h1` 相同的上下节奏。 |
| `h3–h6` | `16px / 26.4px / 700` | 只在长答案内使用；上方 `8px`，下方收紧 `4px`，不再伪造额外字号层级。 |
| paragraph | `16px / 26.4px / 400` | 默认阅读单元；相邻段落只由 `12px` 内容栅格分开。 |
| `hr` | `0.5px content-divider` | 只分隔真正不同的主题，不作为每段装饰；上下各 `12px`、左右各内缩 `6px`，使用 `--border-200-color` 的 62% 透明度，以半像素线得到轻而清晰的分界。 |

- 对话标题不使用 display serif、彩色标题、全大写或过度 letter-spacing。
- Workspace 文件保持较紧凑的 `14px / 1.55`，标题按 `20 / 18 / 16px` 递减；不要把文件预览变成第二套博客主题。

**列表、链接与引用**

- 列表 marker 必须占独立 `32px` 栅格列，正文再留 `8px` 间距；有序列表使用 tabular number，marker 使用 `--text-default`，不是品牌蓝。
- Chat 列表项使用正文相同行高，条目间只增加 `4px + 4px` 的最小节奏；嵌套列表只增加缩进和最小间距，不能形成层层 card。
- 普通链接使用 Nexus 蓝文字；默认保持可辨识，hover / focus 再提供下划线或局部底色。外部链接、workspace 文件、Agent mention 必须保留各自语义图标或标签。
- quote 使用中性 `4px` 左边界、`8px` 左外缩与 `16px` 内缩；背景保持透明，引用正文使用次级文字色，不使用大面积蓝底或强制 italic。
- `mark` 用低饱和暖色或搜索语义色，不能和 error / warning 混用。

### 6.3 Code、表格、媒体与 Artifact

**行内 code 与 kbd**

- inline code 是正文中的术语，不是动作链接：使用 `--font-mono`、`14.4px` 字号、`6px` 圆角、`1px 4px` 内边距、轻中性背景、细边界和统一的 Nexus 蓝墨水。无论是短语、工具名、路径、标识符还是 HTTP 状态码，只要是 inline code 都使用同一个色；代码块则走独立的语法高亮。
- inline code 的蓝色表达“代码主题”，不是可点击性：链接仍以文本链接样式、下划线与 hover / focus 反馈区分；真正可打开的 workspace 文件继续使用带动作语义的蓝色标签。
- kbd 与 inline code 共用紧凑几何，但 kbd 可以保留极轻内侧底边以表达实体按键。

**code block**

- code block 是低对比工具面，不是高饱和暗色“开发者卡片”。light theme 使用偏暖灰或近白表面、细边界、`content` 圆角；dark theme 同样避免强发光和厚阴影。
- 静态 code block 默认不设横向语言栏或行号；语言、复制、折叠或打开等必要操作只在 hover / focus 显现。流式 fence 可保留一个紧凑的运行标记，但不能变成工具栏。
- 代码正文使用 `12–13px / 1.5–1.6` Mono，保留横向滚动、缩进和可复制性；行号只在长代码或文件预览中出现。
- 流式 code fence 在闭合前保持稳定 shell 与轻量 loading 状态，不在每个 token 到达时重排整个代码块。

**table**

- 表格只在比较、参数、结果矩阵或结构数据需要时使用；普通两列解释优先使用 prose 或 definition list。
- table 是正文中的编辑型矩阵：透明底、无外层圆角卡片，只用 `0.5px` 分隔表头和各行；它必须铺满当前阅读列，不能按内容收缩为窄表。滚动壳承担窄屏横向滚动，原生 table 本体使用 `width: 100%` 与 `min-width: max-content` 同时保证铺满与可读性。表头使用正文色的 `700` 字重，不全大写。
- 表格使用 `14px / 1.7`，单元格为 `8px 16px 8px 0`；表头分隔比普通行稍强，不做高饱和 zebra stripe、厚网格或浮雕卡片。
- 窄屏使用受控横向滚动，不能压缩单元格到不可读；表格外层负责滚动，内容渲染器不复制滚动逻辑。

**image、图表与媒体**

- 图片按自然比例展示，最大宽度由阅读列决定；使用 `control` / `content` 圆角和细边界，不套卡片壳。
- caption 使用 `12px` 次级文字，紧贴媒体；图片操作使用 hover 才显现的 ghost / icon button。
- Mermaid、图表和 Office 预览是独立懒加载内容面：加载态保留确定尺寸，错误态展示可恢复说明与动作，不使用抽象装饰插画。

**Artifact**

- Artifact 是产物入口，不是聊天气泡。文件使用单行或两行的紧凑操作行：类型图标、名称、路径摘要与打开动作。
- Artifact 预览只在内容本身有阅读价值时展开；文件集合保持列表，不变成彩色卡片网格。
- 外部打开、下载、reveal 等操作用 icon / ghost button，主路径“打开”才可在 hover 或当前选择时轻度强调为蓝色。

### 6.4 工具过程、问答、权限与错误

**tool / thinking summary**

- 工具调用、搜索、读取、写入和思考摘要默认呈现为单行过程项：状态图标、简短动词、目标摘要和可展开 affordance。
- running 用局部 spinner、细进度或文本状态表达；蓝色只提示当前进行或可继续操作，不绘制大面积蓝色进度卡。
- completed 收敛为低对比结果行；expanded 才显示输入、输出、日志或 code detail。
- thinking 只显示产品允许公开的摘要，不把内部链式推理伪装成内容块；展开行为不得改变已读正文的因果顺序。

**AskUserQuestion 与 permission**

- 它们是用户必须采取行动的唯一强交互 surface：`content` 圆角、单层边界、清楚问题、最少必要选项和一个蓝色提交动作。
- 选项是行级选择器，不是彩色答案卡；已选状态使用浅蓝背景 + 边界 + 选中图标共同表达。
- 提交后收敛成一条完成状态或可展开摘要，不能永久占据与正文同等的注意力。
- 权限解释必须说明将执行什么、作用域和风险；拒绝与取消是 ghost / secondary，不把所有动作都做成 primary。

**error、warning 与 empty state**

- error / warning 是短、具体、可恢复的内容面：问题、影响、下一步动作。语义色只服务图标和关键文本，不铺满整块背景。
- empty state 在工作面只保留一个下一步动作和必要说明；不使用插画、统计卡或大段教学文案填空。

### 6.5 对话身份与消息表面

- User message 使用紧凑、暖中性的低对比 surface 与 Assistant 正文区分：无 Nexus 蓝泡泡、无边框、无阴影；用户内容应有边界但不抢 Assistant 正文。
- User 消息不重复显示用户头像或昵称。日期、重跑、编辑与复制贴在消息下方；Desktop 仅在消息 hover / focus 时显现，窄屏保持可达。
- Assistant / Agent 输出默认无 bubble。展开态身份行使用 32px、8px 圆角头像与垂直居中的名称；正文与身份行保留 8px，时间和模型不抢正文第一行，模型在底部统计中紧跟缓存。底部统计与复制动作始终共享同一行；窄空间依次截断时长、Token、费用与缓存，模型英文名和复制动作保持完整且不换行。运行态与动作按需要贴近对应内容，不把每段回复重复包壳。
- Room 中不同 Agent 的可区分性优先来自名称、头像、顺序与状态；不为每个 Agent 分配独立品牌色。
- system、guide、queue、wake 等控制信息是辅助行，不作为普通发言者，也不与最终答案竞争。
- 轮次级操作（复制、重试、展开）默认在 hover / focus 露出，键盘可达；不得在每段正文旁常驻一排图标。

### 6.6 响应式与动效

- `620px` 以下收紧消息壳、标题和列表列宽；优先保留正文宽度、点击区域、滚动位置和 Composer。
- 窄屏时 rail 进入 overlay 或收拢；主内容相对可用 work plane 居中，不能因 rail 隐藏而跳位。
- 列表 marker 始终占独立列，不用负 margin 压进正文。
- hover / focus 使用 `--motion-duration-fast: 160ms`；面板与 overlay 使用 `--motion-duration-normal: 220ms`；统一 `--motion-ease-standard`。
- 内容流只允许三类动效：新轮次的微弱进入、过程项状态更新、展开 / 收起 detail。正文不做逐字弹跳或大范围 stagger。
- `:focus-visible` 必须保留隔离线与 `--ring`；`prefers-reduced-motion` 下动画近乎即时。

## 7. 实现入口与检查

| 责任 | 文件 |
| --- | --- |
| 主题与语义 token | `web/src/app/styles/theme-tokens.css` |
| 字体、焦点、滚动条、减少动效 | `web/src/app/styles/theme-base.css` |
| surface、dialog、input、Markdown、响应式配方 | `web/src/app/styles/theme-recipes.css` |
| 样式入口与主题变体 | `web/src/app/globals.css` |
| Markdown 与代码 | `web/src/shared/ui/markdown/core/` |
| 代码壳、流式 Markdown 与工作区文件产物 | `web/src/shared/ui/markdown/{code,streaming,workspace}/` |
| 对话内容块、工具过程、问答与 Artifact | `web/src/features/conversation/shared/message/blocks/` |
| 工作区内容与预览 | `web/src/features/conversation/shared/editor/` |

视觉改动前，先确定改动属于哪一层；业务组件只能消费 token 和共享配方。新增或移动视觉基础设施时，同步更新对应目录的 `AGENTS.md`。

### 7.1 迁移顺序

迁移必须自下而上完成，禁止先在某个页面硬编码：

1. 在 `theme-tokens.css` 建立暖中性色、Nexus 蓝色交互状态、控制高度与完整圆角 token。
2. 在 `theme-recipes.css` 收口 button、icon button、input、Composer、menu / popover / dialog、chip 与选择器配方。
3. 在 `shared/ui/` 让基础组件、Markdown、code shell、table 与媒体预览只消费上述语义配方；删除 raw color、渐变、各自不同的阴影与 `rounded-*` 近似值。
4. 在消息块中收口 prose、tool、question、permission、Artifact、error 与流式内容的表面职责；不能为了内容类型复制一套 card 语言。
5. 依次迁移 desktop rail、会话 surface、Composer、workspace、settings 等高频页面；每个页面只做布局和领域内容，不再定义第二套视觉规则。
6. 最后检查 dark / rain、窄屏、键盘焦点、disabled、loading、streaming 与 destructive 状态。

### 7.2 不兼容的旧语言

以下视觉规则与当前 Nexus 设计语言不兼容，应在实际迁移中移除或收口：

- 常规工作面上的三角纹、aura、全局渐变和玻璃拟态。
- 同一屏内多种色相的按钮、badge、icon 背景和进度条。
- 每张 card 都有圆角、描边和阴影的“仪表盘拼贴”。
- 常驻 blue fill、粗描边、双层 shadow、发光 focus 或 oversized pill。
- 同一动作在 header、正文、sidebar 重复出现。
- 用大段辅助说明、彩色标签或强动效弥补信息架构不清楚。
- Assistant 正文外层的聊天 bubble、代码块的重型开发者主题、工具调用的常驻大卡片。
- 把 quote、普通文件引用和每个表格单元都染成品牌蓝；inline code 只保留统一的低饱和 code 蓝墨水，不能把这套强调扩散到普通 prose。

提交视觉改动前确认：

1. 页面能否按“画布 → rail → 工作平面 → 聚焦 surface → 浮层”读懂？
2. 页面是否只有一个主要工作焦点，正文与结果是否强于状态和装饰？
3. 蓝色是否只服务 Nexus 的行动、选择和焦点，而非沦为背景装饰？
4. button、icon button、input、menu、dialog、chip 是否都使用既定高度、圆角与状态配方？
5. Assistant 正文是否无 bubble、无卡片即可清晰阅读；code、table、tool、question、Artifact 是否只在承担独立职责时上浮？
6. 是否使用语义 token，而非复制 raw color、渐变、阴影或 `rounded-*` 近似值？
7. `light` / `dark` / `rain`、窄屏、键盘焦点、disabled、loading、streaming、destructive 与减少动效是否仍可用？

如果删除一个渐变、圆角、标签、卡片或动画后，用户仍能更快判断位置、状态和下一步，默认删除它。
