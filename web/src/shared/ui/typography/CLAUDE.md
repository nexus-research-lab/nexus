# Typography foundation

本目录只管理 App chrome 的语义排版角色。调用方继续选择正确的 `h1 / h2 / p / label / code` 标签，再用 `getUiTypographyClassName` 选择视觉角色；排版 helper 不替代 HTML 语义。

| 角色 | 固定用途 |
| --- | --- |
| `display / featureTitle` | 品牌、欢迎或明确的特性主标题，不进入普通设置卡片 |
| `objectTitle` | 当前 Agent、Room、能力等对象主标题 |
| `pageTitle / sectionTitle` | 页面与内容分区标题 |
| `body / control / supporting` | 普通正文、控件文字、辅助说明 |
| `metadata / caption / overline` | 紧凑元数据、计数和短分组标签 |
| `code` | App chrome 中的短技术标识；代码块仍归 Markdown/Workspace |

- role 固定字体族、字号、行高、默认字重与 tracking；tone 只选择现有语义前景色，有限 `weight` 覆盖只用于同一角色的真实强调。图标、SVG 等不需要字阶的节点使用 `getUiToneClassName`，不得重新拼颜色 utility。
- 外部 `className` 只增加 margin、宽度、换行、截断或布局，不得再次加入 `text-* / leading-* / tracking-* / font-*` 覆盖 recipe。
- `nexus-chat-markdown`、`nexus-workspace-file-markdown`、Launcher 品牌字和 WorkGraph/图形微标签是明确的独立 Surface，不套 App chrome 角色，也不能成为普通业务文字自由写像素值的先例。
