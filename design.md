# Nexus Design System

> 更新日期：2026-07-16

Nexus 是连续协作工具。设计的任务只有三个：让用户知道自己在哪里、谁在工作、下一步是什么。内容优先，材质只建立边界，动效只解释状态变化。

本文是 Nexus 前端视觉与交互规则的唯一入口。协议、数据模型和业务流程仍由 `docs/` 下对应规范负责；视觉规则不再另设一份前端设计文档。

## 1. 范围与原则

适用范围：

- `/app` 工作台、Launcher、桌面 rail、DM / Room、工作区和设置页。
- `shared/ui` 中复用的 surface、dialog、input、button、Markdown 和状态组件。
- `light`、`dark`、`rain` 三个主题及其响应式布局。

基本原则：

1. **结构先于装饰**：导航、摘要、正文、工具状态和结果必须能被快速区分。
2. **一件事一个层级**：同一内容只在一个位置承担主要表达，控制信息贴近目标内容。
3. **状态与动作分离**：状态用文字、图标、状态点或边界表达；动作才使用按钮、菜单或链接。
4. **克制表面**：不靠厚重玻璃、渐变、阴影、流光或缩放制造“高级感”。
5. **共享规则优先**：新增组件先复用现有语义 token 和配方，不在业务 JSX 中复制 raw color。

## 2. 空间模型与页面角色

工作台固定遵循以下层级：

```text
ambient background
  └─ desktop rail
      └─ main content plane
          └─ inset / card / editor surface
              └─ popover / dialog / temporary overlay
```

### 2.1 页面角色

- **Launcher**：启动协作，只保留一个主要焦点，可有更强的入口视觉，但不复制工作台的高密度布局。
- **App**：承载连续工作。rail 负责定位和摘要，主 plane 负责会话、Room 与工作区，不做营销页式构图。
- **Workspace**：文件树、编辑区和预览区是工具界面，内容宽度优先，避免把长文本塞进窄卡片。
- **Overlay**：popover、dialog 和确认浮层只承载当前选择或短流程，关闭后底层结构不应改变。

### 2.2 布局边界

- 工作区详情最大宽度使用 `--workspace-detail-max-width: 980px`。
- 桌面侧栏使用 `desktop-rail`；窄屏沿用现有容器收缩，不另造一套主题。
- 分栏拖拽手柄默认是轻分割线，拖拽或聚焦时才提高对比度。
- rail、头像、分组标题、箭头和摘要共享中心线；侧栏不承载重数据面板。

## 3. Token 合同（当前冻结）

当前 token 保持四层映射，先不改命名、层级和主题值：

1. **主题基础**：`background`、`foreground`、`primary`、`accent`、`border` 等基础值。
2. **环境与材质**：`ambient-*`、`material-*`，只描述主题背景和少量表面边界。
3. **语义表面**：`surface-*`、`modal-*`、`input-shell-*`、`button-*`、`chip-*`。
4. **组件配方**：`desktop-rail`、`surface-panel`、`dialog-shell`、`input-shell` 等 CSS class。

新增组件优先消费第三层或第四层；不得直接复制主题中的 RGBA、渐变和阴影。确需新增 token 时，先写明语义、主题值、替代的旧写法和影响范围，并同步代码与本文。

### 3.1 主题合同

主题由 `:root[data-theme]` 驱动：

| 主题 | 基调 | 主色 | 辅助色 | 文字方向 |
| --- | --- | --- | --- | --- |
| `light` | 透明浅底、冷白材质 | `#5b72ff` | `#4fa29f` | 深蓝灰文字，层次靠透明度和细边界 |
| `dark` | 深海军蓝、低亮度面板 | `#8ea4ff` | `#67d0c2` | 冷白文字，边框和阴影低对比 |
| `rain` | 石板灰、雨幕环境 | `#9ab7ff` | `#74d9cb` | 冷白文字，环境纹理和雨滴保持克制 |

`globals.css` 将 Tailwind 的 `dark` 变体映射到 `dark` 和 `rain`，组件不能只为 `dark` 写一份与 `rain` 冲突的视觉逻辑。

### 3.2 环境背景

- `body` 使用 `--background` 纯色底；环境纹理通过 `--surface-body-background` 独立叠加。
- 三个主题共用等边三角形网格，只改变底色和描边明度。`light` 使用近白底 `#fcfdfc` 与极淡冷灰边线；`dark` 使用低亮 engraved 线条；`rain` 使用更轻的 etched 线条。
- Rain 的雾层、雨滴和水花属于环境层，不能覆盖内容或影响点击。
- 不保留没有消费者的 `ambient-stage-*` 或工作面之外的 `material-*` 渐变 token。页面底色保持纯色，Launcher 的 aura 只属于入口自身。

## 4. 色彩语义

组件表达语义，不把颜色名称当作业务状态名称：

| 语义 | Token | 用途 |
| --- | --- | --- |
| 最强文字 | `--text-strong` | 正文、标题、已确认状态、主要结果 |
| 默认文字 | `--text-default` | 控件、列表标记、次级正文 |
| 弱文字 | `--text-muted` | 摘要、元信息、说明、未激活导航 |
| 极弱文字 | `--text-soft` | 辅助计数、占位提示、非关键装饰 |
| 强图标 | `--icon-strong` | 当前操作、重要工具图标 |
| 默认图标 | `--icon-default` | 常规操作入口 |
| 弱图标 | `--icon-muted` | 辅助图标、未激活状态 |
| 主动作 | `--primary` | 链接、选中、主按钮、焦点状态 |
| 协作辅助 | `--accent` | Agent / Room 辅助状态和次级强调 |
| 分割线 | `--divider-subtle-color` | 默认边界、轻分隔 |
| 强分割线 | `--divider-strong-color` | 明确分区或拖拽边界 |
| 成功 / 警告 / 错误 | `--success` / `--warning` / `--destructive` | 对应状态和不可逆操作 |

普通文字和强调文字尽量共用同一深色基准；强调只增加一个字重等级，不用全局加粗制造层级。

## 5. 字体与排版

### 5.1 字体职责

界面比例字体 `--font-sans`：

```css
-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
"Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC",
ui-sans-serif, sans-serif
```

导航、按钮、表单、侧栏、摘要和其他非内容 UI 使用这条字体链，不依赖未声明的网络字体。侧栏 DM / Room 摘要明确使用 `--font-sans`，不继承中文正文的内容字体。

中文内容字体 `.message-cjk-text`：

```css
"KingHwaOldSong", "PingFang SC", "Hiragino Sans GB",
"Source Han Serif SC", "Songti SC", serif
```

聊天和 Markdown 的 CJK 片段使用该链；`.message-cjk-font` 只负责父级比例字体，不能把整段英文误切成中文字体。

代码字体 `--font-mono`：

```css
"PT Mono", "SFMono-Regular", Menlo, Monaco, Consolas, monospace
```

代码块、行内代码、kbd、源码和纯文本预览使用等宽字体；连续 prose 不使用等宽字体。

### 5.2 字号阶梯

| Token | 值 | 职责 |
| --- | ---: | --- |
| `--font-size-2xs` | 10px | 极小状态、kbd |
| `--font-size-xs` | 11px | 元信息、辅助标签 |
| `--font-size-sm` | 13px | 控件、侧栏、次级内容 |
| `--font-size-base` | 15px | 对话正文和主要阅读内容 |
| `--font-size-md` | 17px | 小标题、选择项 |
| `--font-size-lg` | 22px | 页面级标题 |
| `--font-size-xl` | 28px | 强入口标题 |
| `--font-size-2xl` | 42px | 极少数品牌 / 入口场景 |

组件不得随意扩展字号阶梯。新增字号先改 token，再改组件。

### 5.3 内容密度

- **对话 Markdown**：正文 15px、行高 1.5；标题约 18 / 16 / 15px，使用 `font-medium`；列表项目间距约 `0.125rem`；行内代码紧凑，代码块低对比、轻边界、小圆角。
- **工作区 Markdown**：正文 14px、行高 1.55；标题 18 / 16 / 15px；列表列宽约 `1.65rem`，压缩项目间距和内部段落 margin。
- **源码 / 纯文本**：`message-code-font`，13px、行高 1.6；编辑态、流式态和只读态都优先保证缩进和光标可追踪性。
- **设置和导航**：使用界面字体；组标题要能扫描，条目密度保持紧凑，不用文章字体或大段说明撑高列表。

## 6. 表面与组件规则

### 6.1 表面配方

| 配方 | 角色 | 规则 |
| --- | --- | --- |
| `desktop-rail` | 桌面侧栏 | 透明透出页面纹理，低对比边界 |
| `surface-panel` | 主面板 | 单层填充，轻边界 |
| `surface-inset` | 内容内嵌面 | 透明底、细边界、无外阴影 |
| `surface-card` | 必要内容卡 | 默认透明、细边界；不是默认布局容器 |
| `surface-popover` | 菜单 / 浮层 | 更高不透明度，边界清晰，阴影更深 |
| `chip-default` | 小型状态 / 选择面 | 轻边界、低对比填充 |
| `input-shell` | 输入外壳 | hover / focus 只改变背景、边界和轻微 ring |

工作面不叠加伪元素高光或环境 glow。卡片必须有内容或交互职责，不能用壳体替代层级。

### 6.2 Dialog、圆角和状态

- `dialog-shell`、`dialog-nav`、`dialog-card`、`dialog-input` 和 `dialog-backdrop` 复用 modal / input 语义；弹窗仍是工具界面，不自动变成厚玻璃卡。
- 基础圆角：`--radius-lg: 34px`、`--radius-md: 24px`、`--radius-sm: 16px`。工作区默认使用 `md` 或 `sm`；`xl` 只用于独立认证 / 故障壳。
- 当前项用背景、边界或 primary 的低透明度表达，不依赖粗体堆叠。
- 状态使用文字、轻 badge、状态点或细边界；动作使用 button、menu 或 link。`disabled` 默认使用 `--disabled-opacity: 0.50`，仍需保留结构辨识度。

### 6.3 对话、时间线和工作区

- 正文优先于过程状态；工具、权限、AskUserQuestion、表格和代码块共享轻表面语法。
- 时间线的圆点和轨道只服务顺序；对齐以真实内容锚点为准，不用硬编码 `top`。
- 用户消息先于本轮 Agent 回复；已完成回复按公区发布时间展示，活动 Agent 状态统一收在已完成回复之后。
- 快 Agent 不等待慢 Agent；慢 Agent 只保留一个紧凑状态卡，最终回复到达后替换该状态。
- guide、queue、wake 等控制信息附着在目标卡片或 Thread，不做成完整消息气泡。
- 主 Feed 只展示正文、最终状态和一行摘要；过程细节、完整引导链和队列审计进入 Thread。
- 文件树、编辑区、预览区和分栏边界使用工具语法，不用厚卡片包裹；文件标题和操作 chrome 由共享容器提供，渲染器不重复实现工具栏。

## 7. Markdown、响应式与动效

Markdown 由共享 renderer 负责，页面只提供内容和上下文：

- `remarkMixedScript` 处理 CJK / Latin 分段。
- `message-cjk-font`、`message-cjk-text` 和 `message-code-font` 各司其职。
- `nexus-chat-markdown` 与 `nexus-workspace-file-markdown` 分别负责对话和工作区排版。
- Mermaid 是独立的懒加载内容面，图表工具栏不得混入正文段落。

响应式规则：

- 聊天 feed 使用 `nexus-chat-feed` container；620px 以下收紧消息壳、标题和列表列宽。
- 断点只调整密度，不重新发明字体或颜色系统。
- 小屏优先保留正文宽度、操作可点击性和滚动位置，再压缩装饰与辅助元信息。
- 列表 marker 必须保持独立列，不用负 margin 把序号压进正文。

动效规则：

- 每页最多 2–3 类：入场、悬浮 / reveal、局部过渡。
- `--motion-duration-fast: 160ms` 用于 hover / focus；`--motion-duration-normal: 220ms` 用于面板和布局过渡。
- 统一使用 `--motion-ease-standard: cubic-bezier(0.22, 1, 0.36, 1)`。
- 环境动画不能改变操作反馈；不得用没有信息增量的闪烁、流光、缩放或上浮制造活跃感。
- `:focus-visible` 必须提供背景隔离线与 `--ring` ring；禁止用 `outline: none` 删除唯一焦点提示。
- `prefers-reduced-motion: reduce` 时动画和过渡压缩至近乎瞬时，滚动恢复即时。

## 8. 实现映射与变更规则

| 责任 | 当前文件 |
| --- | --- |
| 主题基础、ambient、material、语义别名 | `web/src/app/styles/theme-tokens.css` |
| 浏览器基线、字体、焦点、滚动条、减少动效 | `web/src/app/styles/theme-base.css` |
| surface / dialog / input / Markdown / 响应式配方 | `web/src/app/styles/theme-recipes.css` |
| 样式入口与主题变体合同 | `web/src/app/globals.css` |
| Markdown 插件、字体分流和代码块 | `web/src/shared/ui/markdown/core/` |
| 工作区文件内容与预览 | `web/src/features/conversation/shared/editor/` |
| 页面、消息密度、时间线和组件边界 | 本文第 2、5、6、7 节 |

修改前检查：

1. 是否能按“背景 → rail → plane → 浮层”读懂页面？
2. 正文是否比状态和装饰更醒目？普通与强调文字是否只拉开一个字重？
3. 中文、英文和代码是否使用正确的字体职责？列表、标题和代码块是否没有额外空白？
4. 组件是否消费语义 token，而不是复制 raw color、渐变或阴影？
5. `light` / `dark` / `rain` 是否保持相同的信息层级？窄屏、键盘焦点和减少动效是否仍可用？

如果删除一个渐变、圆角、标签或动画后，用户仍能更快判断位置、状态和下一步，默认删除它。
