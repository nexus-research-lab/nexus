# 定时任务运行历史

- `use-scheduled-task-run-history-resource.ts` 独占当前 Job 的运行记录、请求身份和加载错误。
- `use-scheduled-task-run-history-actions.ts` 独占复制、重跑、重试投递和释放占用的在途状态；旧 Job 的命令结果不得写回当前弹窗。
- 命令成功与历史刷新是两个结果：刷新失败不得把已经成功的命令反馈改成失败。
- `scheduled-task-run-history-model.ts` 按固定顺序投影运行状态、时长和重跑、重试投递、释放占用动作；`scheduled-task-run-diagnostic-model.ts` 独占诊断行、输出区块与复制文本定义。
- `scheduled-task-run-history-dialog.tsx` 只装配共享 Dialog 壳层、资源、命令与内容视图。
- `view/` 保存内容状态、单项、诊断详情与动作视图，只消费窄接口，不直接请求运行历史或维护命令状态。
