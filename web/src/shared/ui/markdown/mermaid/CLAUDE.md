# Mermaid 图表

- `mermaid-view.tsx`: 组合渲染状态、显示模式和复制反馈。
- `use-mermaid-svg.ts`: 管理 Mermaid 异步渲染生命周期。
- `mermaid-svg-postprocess.ts`: 清理和约束 SVG 输出。
- `mermaid-view-layout.ts`: 容器与 SVG 尺寸纯模型。
- `mermaid-view-parts.tsx`: 源码、预览和状态视图。
- `mermaid-preview-dialog.tsx`: 复用共享模态协议，以无可见标题栏的画布只维护放大预览和拖拽状态。
- `lazy-mermaid-view.tsx`: 延迟加载边界。

主视图不持有弹窗手势状态；SVG 后处理不得访问 React 状态；布局规则只由纯模型定义。
放大预览只保留画布和悬浮关闭动作；可访问标题继续使用视觉隐藏文本，不为显而易见的图表内容增加标题或说明。
模块加载、首次渲染和已有图表更新统一消费 `display/spinner-styles.ts`，状态容器负责 `aria-busy` 与单一 live region；不得在图表分支内复制旋转、尺寸或 reduced-motion class。

放大画布使用具名可聚焦 region，键盘滚动交给浏览器；每实例标题使用独立 ID。指针捕获丢失也必须清理拖拽状态。

源码滚动区同样提供具名 region 与键盘焦点，保留源码空白和浏览器原生滚动，不添加自定义方向键处理。

延迟加载占位和正式图表共用 getMermaidContainerClassName，不复制尺寸模式分支。源码复制反馈使用 useCopyToClipboard，保持 1600ms 时长；计时器清理和卸载后的迟到结果由公共 Hook 处理。

流式更新允许保留上一次有效图等待新图，但清空源码必须立即清除旧 SVG；已经开始的异步渲染在清空、替换或卸载后不得回写。
