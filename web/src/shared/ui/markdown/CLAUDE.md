# Shared Markdown

本目录拥有跨 Feature 复用的 Markdown 渲染能力，不解释 Conversation 轮次或消息内容块协议。

- `markdown-content.tsx` 是静态、流式、正文和摘要的公共入口。
- 文件索引、预览 URL 和打开动作仅通过 `resolveFilePath`、`getFilePreviewUrl` 与单路径 `onOpenWorkspaceFile` 注入；共享入口没有当前 Agent、Session、owner 或 Store 订阅。消费侧必须在能力闭包中固定资源作用域。
- `core/` 负责元素语义、链接、插件和 Fence。
- `streaming/` 负责增量分块与平滑显示。
- `workspace/` 负责文件路径解析和通用文件链接交互；Agent 工作区资源由 `hooks/agent/use-workspace-markdown.ts` 在业务消费侧读取并绑定。
- `mermaid/` 负责图表渲染与预览；标题栏的复制动作和源码/预览模式固定复用 `UiIconButton` 与 `UiSegmentedControl`，不得保留 Mermaid 专属按钮。整张已渲染图是图形几何命中区，但仍使用原生 button 承载点击与键盘，不得用 `div role=button` 和手写 Enter/Space 模拟。
- `code/` 负责静态和流式代码块。
- `code/code-block-content.tsx` 同时导出无壳语法高亮内容供工作区源码预览复用；语义色板、主题选择和等宽字体只保留一份，业务预览不得复制 Prism 配方。

正文 Grid 必须以内容高度逐行顶部排列；外层预览即使提供满高，也只能把剩余空间留在正文末尾，不得把空白平均分配到段落、标题或列表之间。

Markdown `hr` 是正文内部的语义分隔，保持内容列宽度；Conversation 的轮次或说话人边界不得复用它的配方。

Markdown `strong` 统一使用 600 字重；聊天中文正文所用字体只有常规字重，因此只在 `strong` 内允许合成 weight，普通正文继续禁止合成粗体。

会增量追加的段落和引用使用普通换行，禁止 `text-wrap: pretty/balance`；这类算法会随尾部字符到达重新平衡已经展示的行，造成 live 卡片高度反复增减。静态与流式组件保持同一换行契约，terminal 不得再切换排版算法。

Feature 只能通过这里消费通用 Markdown。Conversation 对文件产物、Agent mention/handoff 与 Slash URI 的解释留在自己的适配器中；`createMarkdownComponents` 的 `renderLink` 槽只接收 href/children，返回空值时继续通用安全链接渲染，不得回流领域解释到共享入口。
