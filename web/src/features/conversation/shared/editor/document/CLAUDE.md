# document/ - 文档预览

- `document-file-preview.tsx` 只编排宽度手柄、预览控制器和视图。
- `use-document-preview.ts` 负责下载、离屏渲染、取消和尺寸观察，消费上层 Office scope；路径/账号切换或重试后不得提交旧解析或执行旧任务的延迟测量。
- `document-preview-dom.ts` 集中 docx 渲染产物的页面测量与媒体归一化，不承载 React 状态。
- `document-preview-view.tsx` 只渲染状态、工具栏和预览容器，不发起网络请求或直接调用 docx 解析器。
- Office 文件下载与载荷上限统一经过相邻的 `office-preview-resource.ts`。
- `document-preview-view.tsx` 使用上层统一加载/失败面；加载和失败时均保留测量容器与样式宿主，避免重试 effect 捕获空 ref。未完成的内容必须隐藏并 inert，已成功的文档才进入可访问树；状态展示不得改动 DOCX 纸面样式和缩放规则。
