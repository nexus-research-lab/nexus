# document/ - 文档预览

- `document-file-preview.tsx` 只编排宽度手柄、预览控制器和视图。
- `use-document-preview.ts` 负责下载、渲染、取消和尺寸观察；路径切换后不得执行旧任务的延迟测量。
- `document-preview-dom.ts` 集中 docx 渲染产物的页面测量与媒体归一化，不承载 React 状态。
- `document-preview-view.tsx` 只渲染状态、工具栏和预览容器，不发起网络请求或直接调用 docx 解析器。
- Office 文件下载与载荷上限统一经过相邻的 `office-preview-resource.ts`。
