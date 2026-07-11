# 内容块视图

- `content-renderer.tsx`: 区分 Markdown 与结构化内容并编排时间线。
- `content-renderer-model.ts`: 建立 toolUse/result、任务进度和已消费块索引。
- `content-block-view.tsx`: 按 ContentBlock 判别类型分派具体视图。
- `content-tool-block.tsx`: 适配普通工具、用户问答和权限响应。
- `content-system-event.tsx`: 渲染系统事件与 API 重试倒计时。
- `content-renderer-activity.ts`: 从已归一化投影推导运行状态。
- `content-renderer-timeline.tsx`: 测量并对齐时间线圆点。

内容数组只在纯投影层建立关联；具体块视图不得再次扫描整轮内容或猜测工具归属。
