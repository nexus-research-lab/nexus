# Mermaid 图表

- `mermaid-view.tsx`: 组合渲染状态、显示模式和复制反馈。
- `use-mermaid-svg.ts`: 管理 Mermaid 异步渲染生命周期。
- `mermaid-svg-postprocess.ts`: 清理和约束 SVG 输出。
- `mermaid-view-layout.ts`: 容器与 SVG 尺寸纯模型。
- `mermaid-view-parts.tsx`: 源码、预览和状态视图。
- `mermaid-preview-dialog.tsx`: 放大预览、滚动锁和拖拽状态。
- `lazy-mermaid-view.tsx`: 延迟加载边界。

主视图不持有弹窗手势状态；SVG 后处理不得访问 React 状态；布局规则只由纯模型定义。
