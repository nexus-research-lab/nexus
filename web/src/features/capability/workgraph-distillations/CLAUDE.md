# WorkGraph 能力目录

- `workgraph-distillations-directory.tsx` 是 owner 命名图目录、资源快照、路由和命令编排的唯一入口；详情路由必须直接切换到 `CapabilityDetailPage`，不得继续渲染目录 Header 或搜索框，也不得在纯详情组件重复读取 API、Store 或路由。
- `workgraph-distillation-detail.tsx` 只接收单个 `WorkGraphWorkflow`、可选资源提示与返回、复制、编辑窄动作；内容轴和二级导航复用 `CapabilityDetailPage`，对象标题、说明和动作对齐复用 `CapabilityDetailIdentity`，并以目录同一 `slash_name` 驱动 `UiSeededAvatar`，目标摘要复用共享 Panel 与 Typography。
- 完整图只通过 `WorkGraphWorkflowCanvasPreview` 渲染；能力页只能提供消费面尺寸和语义 Surface 形状，不得复制节点、边或运行状态投影。
- 目录条目只展示 Slash 身份、名称、内置/owner 来源与节点数；完整目标、依赖和验收内容进入详情画布。
- 内置模板只读，不得显示编辑或删除动作；owner 图的 mutation 必须继续遵守 access fence、显式确认与服务端刷新对账；保存后只用服务端已提交工作图替换目录项及其版本。
- 桌面和窄窗都必须允许详情身份与动作分行，保持完整 Button 命中区；不得恢复原生 button、手写字号、任意圆角或页面私有阴影。
- 修改详情结构、动作资格或画布 Surface 时，必须同步维护同目录 DOM 测试和前端基础静态门禁。

- 编辑入口复用 `WorkGraphDistillationDialog`；改名直接在确认表单完成，对话编辑应用后回到表单。目录不持有第二条保存请求或乐观版本投影，只消费已提交工作图回调。

- 删除由 use-workgraph-deletion 持有同步互斥与独立恢复锁；GET 中目标消失才确认删除，目标仍在时 unknown 必须显式开启新操作再确认，accepted/committed 继续只读核对。普通通知或复制反馈不能遮掉未核对删除动作。
- 删除成功使在途目录读取失效，禁止旧列表复活已删除项。编辑准备按路由代次隔离并同步防重，内置模板在处理器再次拒绝编辑/删除。
- 复制复用公共 useCopyToClipboard，仅成功才显示已复制；失败保留图并提供剪贴板提示，计时器由公共 Hook 清理。
