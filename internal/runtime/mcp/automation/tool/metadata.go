package tool

const (
	searchHintListScheduledTasks   = "定时任务 scheduled task list 查看 查询 任务 管理"
	searchHintCreateScheduledTask  = "定时任务 scheduled task create 创建 提醒 每天 每周 定时 cron interval"
	searchHintUpdateScheduledTask  = "定时任务 scheduled task update 编辑 修改 schedule execution reply enabled"
	searchHintDeleteScheduledTask  = "定时任务 scheduled task delete 删除 取消"
	searchHintEnableScheduledTask  = "定时任务 scheduled task enable 启用 恢复"
	searchHintDisableScheduledTask = "定时任务 scheduled task disable 停用 暂停"
	searchHintRunScheduledTask     = "定时任务 scheduled task run now 立即执行 补跑 验证"
	searchHintGetScheduledTaskRuns = "定时任务 scheduled task runs history 运行记录 日志 历史"
)

func searchHintScheduledTaskStatus(enabled bool) string {
	if enabled {
		return searchHintEnableScheduledTask
	}
	return searchHintDisableScheduledTask
}
