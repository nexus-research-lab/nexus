# Nexus Design System

> 更新日期：2026-07-16

本文是 Nexus 前端视觉与交互规则的唯一入口。协议和业务流程仍由 `docs/` 下对应规范负责。

## 1. 设计方向

Nexus 是连续协作工具。界面应让用户快速判断：**我在哪里、谁在工作、下一步是什么**。

- 内容优先：正文和结果强于状态、容器与装饰。
- 层级克制：材质只建立边界，不靠厚玻璃、渐变、阴影或壳体制造层级。
- 状态与动作分离：状态用文字、图标、状态点或边界；动作才使用按钮、菜单或链接。
- 一处表达：同一内容只保留一个主要位置，控制信息贴近目标内容。
- 共享优先：组件使用语义 token 和共享配方，不在业务 JSX 中复制颜色、渐变和阴影。

适用于 Launcher、`/app` 工作台、桌面 rail、DM / Room、工作区、设置页与 `shared/ui`。

## 2. 空间与页面

```text
ambient background
  └─ desktop rail
      └─ main content plane
          └─ inset / card / editor surface
              └─ popover / dialog / temporary overlay
```

- **Launcher**：启动协作，只保留一个主要焦点，可有独立入口视觉。
- **App**：承载连续工作；rail 负责定位和摘要，main plane 负责会话、Room 与工作区。
- **Workspace**：文件树、编辑区和预览区使用工具界面，不套厚卡片。
- **Overlay**：只承载当前选择、确认或短流程，关闭后不改变底层结构。

布局约束：工作区详情最大宽度为 `--workspace-detail-max-width: 980px`；分栏手柄默认是轻分割线；侧栏头像、标题、箭头和摘要共享中心线；窄屏只调整密度，不另造主题。

## 3. Token 与主题

Token 保持四层映射，当前不改命名、层级和主题值：

1. 主题基础：`background`、`foreground`、`primary`、`accent`、`border`。
2. 环境材质：`ambient-*`、`material-*`。
3. 语义表面：`surface-*`、`modal-*`、`input-shell-*`、`button-*`、`chip-*`。
4. 组件配方：`desktop-rail`、`surface-panel`、`dialog-shell`、`input-shell`。

| 主题 | 背景与文字 | 主色 / 辅助色 |
| --- | --- | --- |
| `light` | 近白冷底、深蓝灰文字 | `#5b72ff` / `#4fa29f` |
| `dark` | 深海军蓝、冷白文字 | `#8ea4ff` / `#67d0c2` |
| `rain` | 石板灰、冷白文字 | `#9ab7ff` / `#74d9cb` |

规则：

- `body` 使用 `--background` 纯色底；三角纹由 `--surface-body-background` 独立叠加。
- `light` 背景为 `#fcfdfc`；三种主题共用网格结构，只改变描边明度。
- Rain 的雾、雨滴和水花只属于环境层，不覆盖内容或交互。
- 不新增无消费者的 ambient / material 渐变；Launcher aura 不进入全局工作面。
- 文字使用 `--text-strong/default/muted/soft`，图标使用 `--icon-strong/default/muted`；交互、协作和状态分别使用 `--primary`、`--accent`、`--success/warning/destructive`。

## 4. 字体与密度

| 角色 | 使用 | 禁止 |
| --- | --- | --- |
| `--font-sans` | 导航、按钮、表单、侧栏、摘要、设置 | 依赖未声明的网络字体 |
| `.message-cjk-text` | 聊天和 Markdown 的 CJK 片段 | 应用于整段英文或非内容 UI |
| `--font-mono` | code、kbd、源码、纯文本预览 | 应用于连续 prose |

字体链以 `theme-base.css` 为实现真相源。`.message-cjk-font` 只设置父级比例字体，`remarkMixedScript` 负责 CJK / Latin 分段。

字号阶梯固定为：`2xs 10px`、`xs 11px`、`sm 13px`、`base 15px`、`md 17px`、`lg 22px`、`xl 28px`、`2xl 42px`。组件不得散落新的 `text-[Npx]`。

| 内容 | 正文 / 行高 | 标题与列表 |
| --- | --- | --- |
| 对话 Markdown | 15px / 1.5 | 18 / 16 / 15px；`font-medium`；项目间距约 `0.125rem` |
| 工作区 Markdown | 14px / 1.55 | 18 / 16 / 15px；列表列宽约 `1.65rem` |
| 源码 / 纯文本 | 13px / 1.6 | 保持缩进、行宽和光标可追踪性 |
| 侧栏 / 设置 | `--font-sans` | 组标题可扫描，条目保持紧凑 |

普通文字与强调文字共用深色基准，强调只增加一个字重等级。行内代码紧凑；代码块使用低对比背景、轻边界和小圆角。

## 5. 表面与组件

| 配方 | 规则 |
| --- | --- |
| `desktop-rail` | 透明透出页面纹理，低对比边界 |
| `surface-panel` | 单层填充，轻边界 |
| `surface-inset` / `surface-card` | 透明或低对比底、细边界、无外阴影 |
| `surface-popover` | 更高不透明度、清晰边界、必要阴影 |
| `chip-default` | 轻边界、低对比填充 |
| `input-shell` | hover / focus 只改变背景、边界和轻微 ring |

- 卡片必须有内容或交互职责；当前项用低透明度 primary 背景或边界，不靠粗体堆叠。
- Dialog 复用 modal / input 语义，不能因进入浮层就变成厚玻璃卡。
- 工作区默认使用 `md` / `sm` 圆角；`xl` 仅用于独立认证或故障壳。
- 导航只承担定位和摘要；长内容截断，不把侧栏扩成数据面板。
- 时间线轨道只表达顺序；正文、最终状态和摘要留在 Feed，过程细节进入 Thread。
- guide、queue、wake 等控制信息贴近目标，不做成完整消息气泡；顺序与投影规则见 [`message-processing-spec.md`](docs/specs/message-processing-spec.md)。
- 工具、权限、AskUserQuestion、表格和代码块共享轻表面语法。
- 文件标题和操作 chrome 由共享容器负责，内容渲染器不重复实现工具栏。

## 6. 内容、响应式与动效

- `nexus-chat-markdown` 与 `nexus-workspace-file-markdown` 分别负责对话和工作区排版；Mermaid 是独立的懒加载内容面。
- 620px 以下收紧消息壳、标题和列表列宽；优先保留正文宽度、点击区域和滚动位置。
- 列表 marker 始终占独立列，不用负 margin 压进正文。
- 每页最多保留入场、悬浮 / reveal、局部过渡中的 2–3 类动效。
- hover / focus 使用 `--motion-duration-fast: 160ms`，面板过渡使用 `--motion-duration-normal: 220ms`，统一 easing 为 `--motion-ease-standard`。
- `:focus-visible` 必须保留隔离线与 `--ring`；`prefers-reduced-motion` 下动画近乎即时。
- 颜色不是唯一状态信号；正文不用透明度制造无意义的灰阶。

## 7. 实现入口与检查

| 责任 | 文件 |
| --- | --- |
| 主题与语义 token | `web/src/app/styles/theme-tokens.css` |
| 字体、焦点、滚动条、减少动效 | `web/src/app/styles/theme-base.css` |
| surface、dialog、input、Markdown、响应式配方 | `web/src/app/styles/theme-recipes.css` |
| 样式入口与主题变体 | `web/src/app/globals.css` |
| Markdown 与代码 | `web/src/shared/ui/markdown/core/` |
| 工作区内容与预览 | `web/src/features/conversation/shared/editor/` |

提交视觉改动前确认：

1. 页面能否按“背景 → rail → plane → 浮层”读懂？
2. 正文是否强于状态和装饰？字体、字重、列表与代码密度是否一致？
3. 是否使用语义 token，而非复制 raw color、渐变或阴影？
4. `light` / `dark` / `rain`、窄屏、键盘焦点和减少动效是否仍可用？

如果删除一个渐变、圆角、标签或动画后，用户仍能更快判断位置、状态和下一步，默认删除它。
