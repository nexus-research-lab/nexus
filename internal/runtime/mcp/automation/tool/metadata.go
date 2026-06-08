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
	searchHintSearchTaskHistory    = "定时任务 scheduled task search history 搜索 历史 已删除 候选"
	searchHintGetTaskStatus        = "定时任务 scheduled task status health 状态 健康 失败 恢复"
	searchHintGetTaskEvents        = "定时任务 scheduled task events audit 审计 变更 追踪"
	searchHintDailyReport          = "定时任务 scheduled task daily report 今日 发送情况 投递 报告"
	searchHintRetryDelivery        = "定时任务 scheduled task retry delivery 重试投递 补发 发送失败"
	searchHintRecoverTask          = "定时任务 scheduled task recover stuck 卡住 恢复 释放 占用"
)

func searchHintScheduledTaskStatus(enabled bool) string {
	if enabled {
		return searchHintEnableScheduledTask
	}
	return searchHintDisableScheduledTask
}
