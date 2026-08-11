# Nexus Diagram Design Style Guide

本配置从 `web/src/app/styles/theme-tokens.css`、`theme-base.css`、
`theme-recipes.css` 与 `brand-fonts.css` 提取。浅色图采用 Nexus Sunny 壳层的暖白底、
近黑正文与品牌蓝。图解继续遵守 Diagram Design 的单焦点规则。

## Tokens

### Semantic roles

| Role | Purpose | Light | Dark |
| --- | --- | --- | --- |
| `paper` | 页面背景与节点遮罩 | `#f9f9f7` | `#111822` |
| `paper-2` | 次级表面 | `#f3f3f0` | `#17212c` |
| `ink` | 主文字与主描边 | `#131313` | `#edf3fb` |
| `muted` | 次级文字与默认连线 | `#5f5e5a` | `#a5b4c6` |
| `soft` | 技术副标题与区域标签 | `#6d6b67` | `#8392a5` |
| `rule` | 细分隔线 | `rgba(19,19,19,0.10)` | `rgba(237,243,251,0.12)` |
| `rule-solid` | 强分隔线 | `#e5e5e2` | `#34414f` |
| `accent` | 唯一焦点色 | `#5b72ff` | `#8ea4ff` |
| `accent-tint` | 焦点节点填充 | `rgba(91,114,255,0.08)` | `rgba(142,164,255,0.10)` |
| `link` | HTTP、API 与外部连接 | `#2d5da8` | `#93bbfc` |

浅色模式下，`ink` 对 `paper` 的对比度为 17.63，`muted` 对 `paper` 的对比度为 6.16。

### Series palette

| Token | Light | Dark | Notes |
| --- | --- | --- | --- |
| `series-1` | `#4fa29f` | `#67d0c2` | Nexus 次级数据色 |
| `series-2` | `#59697d` | `#a8b5c7` | 运行状态 |
| `series-3` | `#df9d2e` | `#efb554` | 警告 |
| `series-4` | `#df5d62` | `#f26d77` | 危险 |
| `series-5` | `#898781` | `#97a5b8` | 中性序列 |

### Terminal skin

| Token | Hex | Purpose |
| --- | --- | --- |
| `terminal-page` | `#0d131c` | 页面背景 |
| `terminal-paper` | `#111822` | 终端主体 |
| `terminal-bar` | `#17212c` | 标题栏 |
| `terminal-border` | `#34414f` | 边界 |
| `terminal-ink` | `#edf3fb` | 主文字 |
| `terminal-muted` | `#a5b4c6` | 次级文字 |
| `terminal-soft` | `#8392a5` | 弱文字 |
| `terminal-accent` | `#8ea4ff` | 唯一焦点 |
| `terminal-accent-tint` | `rgba(142,164,255,0.10)` | 焦点填充 |

## Typography

| Role | Family | Size | Weight |
| --- | --- | --- | --- |
| `title` | Instrument Serif | 28px | 400 |
| `node-name` | Geist, system-ui | 12px | 600 |
| `sublabel` | Geist Mono, ui-monospace | 8px | 400 |
| `eyebrow` | Geist Mono, ui-monospace | 8px | 500 |
| `arrow-label` | Geist Mono, ui-monospace | 8px | 400 |

节点名称通过系统无衬线字体回退覆盖中文。技术字段保留等宽排版。标题继续使用
Diagram Design 的编辑式衬线层级，中文由浏览器通用衬线字体回退。

## Geometry

| Token | Value |
| --- | --- |
| `stroke-thin` | `0.8` |
| `stroke-default` | `1` |
| `stroke-strong` | `1.2` |
| `radius-sm` | `4` |
| `radius-md` | `8` |
| `grid` | `4` |
