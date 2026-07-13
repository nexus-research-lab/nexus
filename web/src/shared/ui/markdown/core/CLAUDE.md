# Markdown 核心

- `markdown-code-model.ts`: 将语法节点投影为内联、代码块或 Mermaid 状态。
- `markdown-components.tsx`: 按可判别状态路由正文元素，不承担语义识别。
- `markdown-summary-components.tsx`: 只用于紧凑摘要的内联组件表。
- `markdown-link-model.ts`: 统一链接协议、工作区目标、尾部标点、显示文本和流式 URL 尾部策略。
- `markdown-renderer-shared.tsx`: 组织 React Markdown 插件、受保护区域和共享渲染配置。
- `markdown-fence.ts`: Fence 边界识别。
- `markdown-text-plugins.ts`: 用有序、无状态的节点转换规则处理换行和受控内联 HTML。

摘要可以复用正文组件作为基线，但不得把摘要专用分支塞回正文组件表。代码和链接识别必须先投影为可判别状态，组件只负责状态到视图的路由。URL 规范化必须先经过协议白名单；流式尾部匹配按 Markdown 链接、自动链接、裸链接的优先级执行。文本插件的遍历层只执行替换，不得混入标签配对或正则状态管理。
