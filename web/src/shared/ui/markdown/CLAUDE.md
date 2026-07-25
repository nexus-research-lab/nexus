# Shared Markdown

本目录拥有跨 Feature 复用的 Markdown 渲染能力，不解释 Conversation 轮次或消息内容块协议。

- `markdown-content.tsx` 是静态、流式、正文和摘要的公共入口。
- `core/` 负责元素语义、链接、插件和 Fence。
- `streaming/` 负责增量分块与平滑显示。
- `workspace/` 负责 Agent 工作区路径解析和文件打开适配。
- `mermaid/` 负责图表渲染与预览。
- `code/` 负责静态和流式代码块。
- `code/code-block-content.tsx` 同时导出无壳语法高亮内容供工作区源码预览复用；语义色板、主题选择和等宽字体只保留一份，业务预览不得复制 Prism 配方。

正文 Grid 必须以内容高度逐行顶部排列；外层预览即使提供满高，也只能把剩余空间留在正文末尾，不得把空白平均分配到段落、标题或列表之间。

Feature 只能通过这里消费通用 Markdown。Conversation 对文件产物等消息协议的解释留在自己的适配器中，不得回流到共享入口。
