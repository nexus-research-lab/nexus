// INPUT: Goal 工具语义关键词。
// OUTPUT: 工具检索使用的稳定 search hints。
// POS: Goal MCP 元数据真相源。
package tool

const (
	searchHintGetGoal      = "goal current status budget usage remaining tokens 当前目标 状态 预算 用量"
	searchHintCreateGoal   = "goal create start objective token_budget long running task 创建 启动 长程目标 预算"
	searchHintRetargetGoal = "goal retarget correct replace objective explicit user correction 更正 修正 替换 当前目标"
	searchHintUpdateGoal   = "goal update complete blocked finish completion audit 标记完成 阻塞 审计"
)
