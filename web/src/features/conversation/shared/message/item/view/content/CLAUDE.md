# 内容块视图

- `content-renderer.tsx`: 只区分 Markdown 与结构化内容入口。
- `content-renderer-contract.ts`: 定义入口与结构化编排器共同消费的窄属性契约。
- `structured-content-renderer.tsx`: 建立一次内容投影并编排块视图、时间线和流式活动状态。
- `content-renderer-model.ts`: 建立 toolUse/result、任务进度和已消费块索引。
- `content-block-view.tsx`: 通过穷尽注册表分派 ContentBlock，并拥有空节点和时间线框架。
- `content-tool-block.tsx`: 分阶段适配普通工具、用户问答和权限响应。
- `content-system-event.tsx`: 渲染系统事件与 API 重试倒计时。
- `content-renderer-timeline.tsx`: 测量并对齐时间线圆点。

内容数组只在纯投影层建立关联；具体块视图不得再次扫描整轮内容或猜测工具归属。
新增内容块类型必须同时进入穷尽渲染注册表，禁止在编排器中追加类型分支。
内容投影向相邻 `activity/` 提供已消费块、已结束工具和隐藏工具集合；活动领域不得反向依赖本目录的视图模型。
DOM 锚点测量和系统事件样式属于具体视图，不得回流到消息领域模型。
