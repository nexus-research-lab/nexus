// INPUT: Automation 调度器签发的任务、运行与权限续跑身份。
// OUTPUT: DM/Room runtime 与 Execution MCP 共用的不可由模型参数构造的 run binding。
// POS: Automation 控制面到 runtime 的可信身份协议；正文和 session_key 均不承担授权语义。
package protocol

import "strings"

// AutomationRunContext 描述当前 runtime round 所属的定时任务运行。
//
// 该结构只由 Nexus 后端调度链路写入。模型可在隐藏上下文中读取这些字段，
// 但 Automation MCP 的访问范围直接使用本结构，而不解析提示词或会话名称。
type AutomationRunContext struct {
	JobID                    string
	RunID                    string
	JobName                  string
	PermissionPolicyRevision int
	ResumeToolName           string
	ResumeResourceScope      string
}

// Normalized 返回去除传输噪声后的副本。
func (c AutomationRunContext) Normalized() AutomationRunContext {
	result := c
	result.JobID = strings.TrimSpace(result.JobID)
	result.RunID = strings.TrimSpace(result.RunID)
	result.JobName = strings.TrimSpace(result.JobName)
	result.ResumeToolName = strings.TrimSpace(result.ResumeToolName)
	result.ResumeResourceScope = strings.TrimSpace(result.ResumeResourceScope)
	return result
}

// Valid 表示该 binding 足以作为一次可信 Automation run 身份。
func (c AutomationRunContext) Valid() bool {
	normalized := c.Normalized()
	return normalized.JobID != "" && normalized.RunID != ""
}
