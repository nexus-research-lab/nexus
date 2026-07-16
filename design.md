# Nexus Design System

> 更新日期：2026-07-16

## 0. 设计立场

Nexus 面向连续协作。界面首先回答三个问题：我在哪里、谁在工作、下一步是什么。背景和材质只负责建立边界，不能与正文竞争。

- 导航和摘要用于定位会话、Room、能力与设置。
- 对话和时间线用于读取正文、工具状态和结果。
- 工作区和文件用于查看、编辑和验证正在修改的内容。
- 浮层和弹窗只承载当前选择、确认或短流程。

动效只用于状态变化、焦点移动和局部反馈。没有信息增量的闪烁、流光、悬浮或缩放不应进入工作面。

## 1. 适用范围与文档边界

### 1.1 适用范围

- `/app` 主工作界面。
- 桌面 rail、会话导航、DM / Room、对话消息、工作区文件预览。
- `shared/ui` 中复用的 surface、dialog、input、button、Markdown 和状态组件。
- `light`、`dark`、`rain` 三个当前主题。

### 1.2 与其他文档的关系

- 本文档：视觉 token、字体、材质、排版和组件配方。
- [`docs/specs/frontend-design-spec.md`](docs/specs/frontend-design-spec.md)：页面角色、消息密度、时间线和组件边界。
- [`web/src/app/styles/theme-tokens.css`](web/src/app/styles/theme-tokens.css)：token 的实际定义和主题值。
- [`web/src/app/styles/theme-base.css`](web/src/app/styles/theme-base.css)：浏览器基线、字体链、焦点和滚动条。
- [`web/src/app/styles/theme-recipes.css`](web/src/app/styles/theme-recipes.css)：surface、dialog、Markdown 和响应式配方。

如果本文与代码不一致，以当前代码为准；修正实现后应同步修正文档。

## 2. 页面与空间层级

`/app` 固定采用四层空间模型：

```text
ambient background
  └─ desktop rail
      └─ main content plane
          └─ inset / card / editor surface
              └─ popover / dialog / temporary overlay
```

### 2.1 Launcher

Launcher 负责启动协作，可以有更强的入口视觉，但只保留一个主要焦点。它不应把 `/app` 的高密度工具布局复制到首屏。

### 2.2 App

`/app` 负责连续工作：

- rail 负责导航和摘要，不承载重数据面板。
- 主 plane 负责会话、Room 和工作区。
- inset / card 只用于必要的内容分组或交互状态。
- 浮层必须能被识别为暂时性操作，不改变底层页面结构。

### 2.3 布局边界

- 工作区详情最大宽度使用 `--workspace-detail-max-width: 980px`。
- 桌面侧栏使用 `desktop-rail`，窄屏通过现有布局和容器规则收缩，不额外创建一套主题。
- 分栏拖拽手柄表现为轻分割线；可见时才显示方向提示。
- 内容区优先保证可读宽度，避免把长文本压成过窄的卡片。

## 3. Token 分层（当前不改）

现有 token 不是单层颜色表，而是从基础主题到组件语义的四层映射：

1. **主题基础**：`background`、`foreground`、`primary`、`accent`、`border` 等基础值。
2. **环境与材质**：`ambient-*`、`material-*`，描述主题背景和少量表面边界。
3. **语义表面**：`surface-*`、`modal-*`、`input-shell-*`、`button-*`、`chip-*`。
4. **组件配方**：`desktop-rail`、`surface-panel`、`dialog-shell`、`input-shell` 等 CSS class。

新增组件应优先消费第三层语义 token 或第四层配方，不应直接复制主题中的 RGBA、渐变和阴影。

### 3.1 主题合同

主题由 `:root[data-theme]` 驱动：

| 主题 | 基调 | 主色 | 辅助色 | 文字方向 |
| --- | --- | --- | --- | --- |
| `light` | 透明浅底、冷白材质 | `#5b72ff` | `#4fa29f` | 深蓝灰文字，层次靠透明度和细边界 |
| `dark` | 深海军蓝、低亮度面板 | `#8ea4ff` | `#67d0c2` | 冷白文字，边框和阴影低对比 |
| `rain` | 石板灰、雨幕环境 | `#9ab7ff` | `#74d9cb` | 冷白文字，环境纹理和雨滴保持克制 |

`globals.css` 将 Tailwind 的 `dark` 变体映射到 `dark` 和 `rain`，因此组件不能只为 `dark` 写一份与 `rain` 冲突的视觉逻辑。

### 3.2 环境背景

`body` 使用 `--background` 作为纯色底，并通过 `--surface-body-background` 单独叠加 ambient pattern。当前背景纹理是同一套等边三角形网格，通过主题改变底色和描边明度：

- light：`#fcfdfc` 近白底色配白色高光和极淡冷灰底线组成的 embossed 纹理，清晰但不抢正文。
- dark：深色 engraved 线条，保持精密感但不抢文字。
- rain：更轻的 etched 线条，与雨滴 canvas 分开工作。

Rain 主题额外使用低频雾层、三层雨滴和水花；这些是环境层，不得覆盖内容层，也不得影响点击。

不保留没有消费者的 `ambient-stage-*` 或工作面之外的 `material-*` 渐变 token；页面底色使用主题纯色，Launcher 只保留入口 Hero 自身的克制 aura，不影响全局页面。公开 Landing 预览使用现有 card 填充和本地边界/阴影，工作区的 popover、input、canvas 等功能表面仍按语义配方取值。

## 4. 色彩语义

组件只表达语义，不把颜色名称当作业务状态名称。

| 语义 | Token | 用途 |
| --- | --- | --- |
| 最强文字 | `--text-strong` | 正文、标题、已确认状态、主要结果 |
| 默认文字 | `--text-default` | 控件、列表标记、次级正文 |
| 弱文字 | `--text-muted` | 摘要、元信息、说明、未激活导航 |
| 极弱文字 | `--text-soft` | 辅助计数、占位提示、非关键装饰 |
| 强图标 | `--icon-strong` | 当前操作、重要工具图标 |
| 默认图标 | `--icon-default` | 常规操作入口 |
| 弱图标 | `--icon-muted` | 辅助图标、未激活状态 |
| 主动作 | `--primary` | 链接、选中、主按钮、焦点相关状态 |
| 协作辅助 | `--accent` | agent / Room 辅助状态和次级强调 |
| 分割线 | `--divider-subtle-color` | 默认边界、轻分隔 |
| 强分割线 | `--divider-strong-color` | 明确分区或拖拽边界 |
| 成功 | `--success` | 已完成、同步、可用 |
| 警告 | `--warning` | 需要注意但不阻断 |
| 错误 | `--destructive` | 失败、删除、不可逆操作 |

文字对比优先通过语义颜色解决，不通过把所有文字加粗解决。普通正文和强调文字应尽量共用同一深色基准，强调只增加一个字重等级。

## 5. 字体与排版

### 5.1 字体职责

当前系统有三个明确的字体角色：界面比例字体、内容中文字体和代码等宽字体。它们不是可随意互换的装饰选择。

#### 界面比例字体：`--font-sans`

```css
-apple-system,
BlinkMacSystemFont,
"Segoe UI",
"PingFang SC",
"Hiragino Sans GB",
"Microsoft YaHei",
"Noto Sans CJK SC",
ui-sans-serif,
sans-serif
```

界面不依赖未声明的网络字体。macOS 使用系统 sans，Windows 使用 Segoe UI，中文按系统字体回退。导航、按钮、表单、侧栏、摘要和非内容 UI 使用这条字体链。

#### 中文内容字体：`.message-cjk-text`

聊天和 Markdown 里的中文片段使用当前声明的京华老宋体链，英文仍由父级比例字体承载：

```css
"KingHwaOldSong",
"PingFang SC",
"Hiragino Sans GB",
"Source Han Serif SC",
"Songti SC",
serif
```

`.message-cjk-font` 只负责让父级使用界面比例字体；`remarkMixedScript` 将 CJK 片段包成 `.message-cjk-text`。不要把 `.message-cjk-font` 误当成“整段都使用中文字体”。

#### 代码字体：`--font-mono`

```css
"PT Mono",
"SFMono-Regular",
Menlo,
Monaco,
Consolas,
monospace
```

代码块、行内代码、kbd、源码/纯文本预览使用等宽字体。连续 prose 不使用等宽字体；等宽字体会把英文单词切成机械网格，破坏中英混排节奏。

### 5.2 字号阶梯

当前 `@theme inline` 保留以下字号语义：

| Token | 值 | 典型职责 |
| --- | ---: | --- |
| `--font-size-2xs` | 10px | 极小状态、kbd |
| `--font-size-xs` | 11px | 元信息、辅助标签 |
| `--font-size-sm` | 13px | 控件、侧栏、次级内容 |
| `--font-size-base` | 15px | 对话正文和主要阅读内容 |
| `--font-size-md` | 17px | 小标题、选择项 |
| `--font-size-lg` | 22px | 页面级标题 |
| `--font-size-xl` | 28px | 强入口标题 |
| `--font-size-2xl` | 42px | 仅用于极少数品牌/入口场景 |

不在组件中为了局部视觉任意扩展字号阶梯。需要新字号时先提出 token 变更，而不是散落新的 `text-[Npx]`。

### 5.3 当前内容排版

#### 对话 Markdown

- 基础正文：15px，行高 1.5。
- 标题：约 18 / 16 / 15px，使用 `font-medium`，不使用厚重的 `font-bold`。
- 普通正文和强调文字使用同一深色文字基准；强调只提升一级字重。
- 列表采用固定 marker 栅格，正文列不被序号压住；聊天区项目间距约 0.125rem。
- 行内代码使用紧凑的等宽 pill；代码块使用轻边框、低对比背景和克制圆角。

#### 工作区 Markdown

- 基础正文：14px，行高 1.55，适合窄分栏中的连续阅读。
- 标题：18 / 16 / 15px，标题上下间距小于默认文档样式。
- 列表列宽约 1.65rem，项目间距和列表项内部段落 margin 都被收紧。
- 引用、图片和表格保留必要分组空间，但不使用聊天区的极紧节奏。

#### 工作区纯文本 / 源码

- 使用 `message-code-font`。
- 当前预览字号 13px，行高 1.6。
- 编辑态、实时写入态和只读源码态都优先保持行宽、缩进和光标可追踪性。

#### 侧栏 DM / Room 摘要

- 使用 `--font-sans`，不继承正文的 CJK 字体。
- 摘要是导航信息，不应呈现为一段具有文章气质的内容。
- 摘要中的代码仍保留代码语义，不为了统一字体而改成界面字体。

## 6. 表面与材质配方

### 6.1 Surface 原语

| CSS 配方 | 角色 | 当前行为 |
| --- | --- | --- |
| `desktop-rail` | 桌面侧栏底面 | 透明透出页面纹理、低对比边界 |
| `surface-panel` | 主面板 | 单层卡片填充、轻边界 |
| `surface-inset` | 内容内嵌面 | 透明底、细边界、无外阴影 |
| `surface-card` | 必要的内容卡 | 默认透明、细边界、无阴影；不是默认布局容器 |
| `surface-popover` | 菜单和浮出层 | 不透明度更高、边界清晰、阴影更深 |
| `chip-default` | 小型状态/选择面 | 轻边界和低对比填充 |
| `input-shell` | 输入外壳 | hover / focus 只改变背景、边界和轻微 ring |

工作面不叠加伪元素高光或环境 glow。页面底色保持纯色，环境纹理和入口装饰独立于正文，不能覆盖内容或交互。

### 6.2 Dialog 原语

- `dialog-shell`：浮层主体，使用 modal surface token。
- `dialog-nav`：弹窗内部导航，使用轻边界。
- `dialog-card`：可选项或设置分组，默认无阴影。
- `dialog-card-active`：只用 primary 的低透明度背景和边界表示选中。
- `dialog-input`：与 `input-shell` 同样遵循 focus-within 规则。
- `dialog-backdrop`：遮罩只负责隔离背景，不抢前景内容。

弹窗内部仍是工具界面，不应因为进入 modal 就变成厚重玻璃卡片。

### 6.3 圆角合同

主题基础 token：

- `--radius-lg: 34px`
- `--radius-md: 24px`
- `--radius-sm: 16px`

配方层额外提供：`surface-radius-xl: 34px`、`surface-radius-lg: 24px`、`surface-radius-md: 18px`、`surface-radius-sm: 14px`。工作区卡片默认使用 `md` 或 `sm`；`xl` 只用于独立的认证/故障壳，不用于连续内容网格。

## 7. 组件规则

### 7.1 导航与侧栏

- 侧栏的第一职责是定位，不是展示完整数据。
- 分组标题、头像、箭头和摘要共用中心线。
- 当前项用背景、边界或 primary 的低透明度表达，不依赖粗体堆叠。
- DM / Room 摘要保持界面字体和短行高；长内容通过截断和 tooltip 处理。

### 7.2 对话与时间线

- 正文优先于过程状态；状态行必须紧凑。
- 工具、权限、AskUserQuestion、表格和代码块共享轻表面语法。
- 时间线轨道服务于顺序，不抢正文对比度。
- 同一内容只出现一次；guide、queue、wake 等控制信息附着在目标位置。

### 7.3 工作区与文件预览

- 文件树、编辑区和预览区是工具界面，不使用厚卡片包裹。
- Markdown 预览使用内容字体规则；源码预览使用等宽字体；HTML、PDF、Office 和图片保留各自原生预览语义。
- 文件标题、元信息、下载和聚焦操作由统一 chrome 承担，内容渲染器不重复实现工具栏。
- 分栏边界使用 `divider-subtle`；拖拽时才提高可见度。

### 7.4 状态与动作

- 状态：文字、轻 badge、状态点、细边界。
- 动作：button、menu、link。
- 不用 status 色同时承担按钮底色、正文和装饰背景。
- 不用没有信息增量的流光、缩放或上浮制造“活跃感”。
- disabled 默认使用 `--disabled-opacity: 0.50`，并保留可辨识的结构。

## 8. Markdown 与代码

Markdown 渲染由共享 renderer 负责，业务页面只提供内容和上下文：

- `remarkMixedScript` 处理 CJK / Latin 分段。
- `message-cjk-font` 提供父级比例字体。
- `message-cjk-text` 提供中文内容字体。
- `message-code-font` 只用于 code、kbd、源码和代码表面。
- `nexus-chat-markdown` 负责对话排版；`nexus-workspace-file-markdown` 负责工作区文件排版。
- Mermaid 是独立的懒加载内容面，不应把图表工具栏混进正文段落。

代码表面应当“能看懂但不抢戏”：背景比面板略深或略浅，边界轻，圆角小于普通面板，代码字号低于正文，避免巨大阴影和高饱和色块。

## 9. 响应式与密度

- 聊天 feed 使用 `nexus-chat-feed` container；在 620px 以下收紧消息壳层、标题和列表列宽。
- 断点调整的是内容密度，不是重新发明一套字体或颜色系统。
- 小屏优先保留正文宽度、操作可点击性和滚动位置，再压缩装饰和辅助元信息。
- 列表 marker 必须在窄列中仍保持独立列，不允许通过负 margin 把序号压进正文。

## 10. 动效、焦点与可访问性

### 10.1 动效 token

- `--motion-duration-fast: 160ms`：hover、focus、按钮和局部状态。
- `--motion-duration-normal: 220ms`：面板、颜色和布局级过渡。
- `--motion-ease-standard: cubic-bezier(0.22, 1, 0.36, 1)`：统一的进入和回弹节奏。

Rain / Sunny 环境层可以使用更长的环境动画，但不应改变操作反馈的时间合同。工作区状态优先使用文字、图标或静态状态点。

### 10.2 焦点

全局 `:focus-visible` 使用“背景隔离线 + ring”双层提示，颜色来自 `--ring`。组件不得通过 `outline: none` 删除焦点而不提供等价提示。

### 10.3 减少动效

`prefers-reduced-motion: reduce` 时，所有动画和过渡被压缩到近乎瞬时，滚动恢复为即时行为。新增动画必须自动继承这一规则。

### 10.4 阅读与对比

- 正文使用 `--text-strong` 或其明确语义继承，不用透明度制造无意义的“灰正文”。
- `--text-muted` 只用于元信息、摘要和辅助说明。
- 颜色不是唯一状态信号；状态应同时有文字、图标、边界或位置变化。
- 滚动条使用细轨道和低对比 thumb，不能遮挡内容。

## 11. 实现映射

| 责任 | 当前文件 |
| --- | --- |
| 主题基础、ambient、material、语义别名 | `web/src/app/styles/theme-tokens.css` |
| 浏览器基线、字体、焦点、滚动条、减少动效 | `web/src/app/styles/theme-base.css` |
| surface / dialog / input / Markdown / 响应式配方 | `web/src/app/styles/theme-recipes.css` |
| 样式入口与主题变体合同 | `web/src/app/globals.css` |
| Markdown 插件、内容字体和代码分流 | `web/src/shared/ui/markdown/core/markdown-renderer-shared.tsx`、`markdown-text-plugins.ts` |
| Markdown 元素组件和代码块 | `web/src/shared/ui/markdown/core/markdown-components.tsx`、`web/src/shared/ui/markdown/code/` |
| 工作区文件内容 | `web/src/features/conversation/shared/editor/text/text-file-content.tsx` |
| 工作区文件标题、工具栏和预览路由 | `web/src/features/conversation/shared/editor/workspace-file-preview-*.tsx` |
| 页面/时间线/组件边界 | `docs/specs/frontend-design-spec.md` |

## 12. Token 冻结期间的修改规则

在明确开始下一轮 token 设计前：

1. 不新增同义颜色、同义 surface 或同义字号 token。
2. 不在业务组件里复制主题 RGBA、渐变和阴影；优先使用语义 alias。
3. 不把 `--font-sans`、`.message-cjk-text`、`.message-code-font` 混成一个“万能字体”。
4. 不通过局部 `font-bold`、透明度或负 margin 修复层级问题；先检查颜色、字重、行高和结构是否各司其职。
5. 新组件先选择现有 surface recipe，再决定是否需要新的配方。
6. 如果确实需要新 token，先在设计评审中说明：语义、主题值、替代了哪些旧写法、影响哪些页面；本文件和 `theme-tokens.css` 同步更新。

## 13. 交付前检查表

- [ ] 页面是否仍然能按“背景 → rail → plane → 浮层”读懂？
- [ ] 正文是否比状态和装饰更醒目？
- [ ] 普通文字与加粗文字是否只在一个维度上拉开？
- [ ] 中文、英文、代码是否使用了各自的字体职责？
- [ ] 列表、标题和代码块是否没有额外的空白撑开内容？
- [ ] 组件是否消费语义 token，而不是复制 raw color？
- [ ] light / dark / rain 是否都保持相同的信息层级？
- [ ] 键盘焦点、减少动效和窄屏滚动是否仍然可用？
- [ ] 新增的视觉规则是否已经写回对应 recipe，而不是散落在页面 JSX？

## 14. 验收原则

如果删除一个渐变、圆角、标签或动画后，用户仍能更快判断位置、状态和下一步，那么默认删除它。
