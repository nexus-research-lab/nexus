# Markdown 核心

- `markdown-components.tsx`: 正文元素和代码块组件表。
- `markdown-summary-components.tsx`: 只用于紧凑摘要的内联组件表。
- `markdown-link-model.ts`: 外链协议、尾部标点和显示文本纯模型。
- `markdown-renderer-shared.tsx`: React Markdown 插件与共享渲染配置。
- `markdown-fence.ts`: Fence 边界识别。
- `markdown-text-plugins.ts`: 用有序、无状态的节点转换规则处理换行和受控内联 HTML。

摘要可以复用正文组件作为基线，但不得把摘要专用分支塞回正文组件表。URL 规范化必须先经过协议白名单。文本插件的遍历层只执行替换，不得混入标签配对或正则状态管理。
