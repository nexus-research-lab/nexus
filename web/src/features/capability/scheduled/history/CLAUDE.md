# 定时任务运行历史

- `use-scheduled-task-run-history-resource.ts` 独占当前 owner+Job 的运行记录、请求身份和加载错误；旧 scope 请求不仅不能写视图，也必须以 superseded 结束，禁止把旧运行数组返回给对账调用方。
- `use-scheduled-task-run-history-actions.ts` 独占复制、重跑、重试投递、释放占用确认目标和在途状态；状态代次绑定 owner+Job，旧 scope 或旧 Job 的命令结果不得写回当前弹窗。
- 命令成功与历史刷新是两个结果：刷新失败不得把已经成功的命令反馈改成失败。
- `scheduled-task-run-history-model.ts` 按固定顺序投影运行状态、时长和重跑、重试投递、释放占用动作；`scheduled-task-run-diagnostic-model.ts` 独占诊断行、输出区块与复制文本定义。
- 任务处于 `deleting` 或 `review_required` 时历史仍可读取和刷新，但重跑、重试投递与释放占用都必须禁用；目录事件 handler 同时保留第二道 guard。
- `scheduled-task-run-history-dialog.tsx` 只装配共享 Dialog 壳层、资源、命令与内容视图。
- `view/` 保存内容状态、单项、诊断详情与动作视图，只消费窄接口，不直接请求运行历史或维护命令状态。
- 历史弹窗用任务名和真实状态命名，不在标题区显示 Job ID 或“运行历史”套话；Job/Run 等内部身份仅保留在折叠诊断，空态只说明尚无记录。
- 释放运行占用先经共享决策框确认；可见文案只说明会取消本次运行并允许后续重跑，不显示内部 Run ID。
