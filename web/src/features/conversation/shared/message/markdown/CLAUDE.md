# Markdown 渲染

- `markdown-renderer.tsx` 与 `markdown-renderer-content.tsx` 只负责公开入口和渲染编排。
- `core/` 负责正文、摘要、链接、插件和 Fence 等稳定语义。
- `streaming/` 负责增量内容分块与平滑展示。
- `workspace/` 负责 Agent 工作区路径解析和文件打开动作。
- `mermaid/` 负责图表渲染、模式切换和放大预览。

正文、流式、工作区和 Mermaid 不得通过根目录散落文件互相穿透；新增逻辑放入实际拥有状态或协议的子域。
