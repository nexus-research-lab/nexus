# WorkGraph 能力目录

- `workgraph-distillations-directory.tsx` 是 owner 命名图目录、资源快照、路由和命令编排的唯一入口；不得在纯详情组件重复读取 API、Store 或路由。
- `workgraph-distillation-detail.tsx` 只接收单个 `WorkGraphWorkflow` 与返回、复制、编辑窄动作；对象标题、说明、目标摘要和动作必须复用共享 Typography、Panel 与 Button。
- 完整图只通过 `WorkGraphWorkflowCanvasPreview` 渲染；能力页只能提供消费面尺寸和语义 Surface 形状，不得复制节点、边或运行状态投影。
- 目录条目只展示 Slash 身份、名称、内置/owner 来源与节点数；完整目标、依赖和验收内容进入详情画布。
- 内置模板只读，不得显示编辑或删除动作；owner 图的 mutation 必须继续遵守 access fence、显式确认与服务端刷新对账。
- 桌面和窄窗都必须允许详情身份与动作分行，保持完整 Button 命中区；不得恢复原生 button、手写字号、任意圆角或页面私有阴影。
- 修改详情结构、动作资格或画布 Surface 时，必须同步维护同目录 DOM 测试和前端基础静态门禁。
