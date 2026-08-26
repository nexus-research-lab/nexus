// INPUT: 服务端签发的 AutomationRunContext。
// OUTPUT: 模型可见但用户不可见的任务运行与权限续跑说明。
// POS: Automation trusted identity 到 runtime contextual input 的唯一文本投影。
package runtime

import (
	"fmt"
	"html"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AutomationRunContextualInputs 把可信 run binding 投影为隐藏上下文。
// 工具权限不从这里解析；该文本只帮助模型理解本轮运行语义。
func AutomationRunContextualInputs(binding *protocol.AutomationRunContext) []ContextualInputBlock {
	if binding == nil {
		return nil
	}
	normalized := binding.Normalized()
	if !normalized.Valid() {
		return nil
	}
	attributes := []string{
		fmt.Sprintf(`job_id="%s"`, html.EscapeString(normalized.JobID)),
		fmt.Sprintf(`run_id="%s"`, html.EscapeString(normalized.RunID)),
	}
	if normalized.JobName != "" {
		attributes = append(attributes, fmt.Sprintf(`task_name="%s"`, html.EscapeString(normalized.JobName)))
	}
	if normalized.PermissionPolicyRevision > 0 {
		attributes = append(attributes, fmt.Sprintf(`permission_revision="%d"`, normalized.PermissionPolicyRevision))
	}
	body := "This turn is a scheduled-task run. The scheduler owns result delivery; return only the requested result and do not address or route the destination yourself. nexus.command is read-only and already scoped to this task."
	if normalized.ResumeToolName != "" {
		body += fmt.Sprintf(
			" The user approved a previous permission request. Call tool %q again with the task's original arguments",
			normalized.ResumeToolName,
		)
		if normalized.ResumeResourceScope != "" {
			body += fmt.Sprintf(" for resource %q", normalized.ResumeResourceScope)
		}
		body += "; wait for its actual result before summarizing."
	}
	content := fmt.Sprintf(
		"<nexus_automation_context %s>\n%s\n</nexus_automation_context>",
		strings.Join(attributes, " "),
		body,
	)
	return []ContextualInputBlock{
		NewContextualInputBlock(ContextualInputNameAutomation, content, 0, map[string]string{
			"job_id": normalized.JobID,
			"run_id": normalized.RunID,
		}),
	}
}
