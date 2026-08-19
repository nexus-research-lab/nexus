// INPUT: Objective Alignment 的稳定工具名与结构化 report_json 约束。
// OUTPUT: Goal continuation 与未来 guard evaluator 可嵌入的统一审计提示。
// POS: 共享判定协议的模型说明真相源，不包含 Goal 或 loop 生命周期规则。
package objectivealignment

import "strings"

const ReportJSONDescription = "One JSON object with decision (aligned, not_aligned, or inconclusive), criteria_results covering every authoritative criterion exactly once, and a summary. Each criterion result has criterion, status (satisfied, unsatisfied, or inconclusive), evidence [{ref, claim}], and an optional gap. Satisfied requires evidence; unsatisfied or inconclusive requires a gap."

// PromptContract 返回判定方必须遵守的稳定协议。
func PromptContract() string {
	return strings.TrimSpace(`
Objective alignment contract:
- Treat the objective and completion criteria supplied by the backend as authoritative; do not rewrite, narrow, add, or omit criteria.
- Inspect current authoritative state before deciding. For every criterion, return exactly one result.
- Use status "satisfied" only with at least one reviewable evidence ref and a precise claim it proves.
- Use status "unsatisfied" when current evidence confirms a concrete gap, and state that gap.
- Use status "inconclusive" when evidence is missing, weak, indirect, stale, or contradictory; state what evidence is needed.
- The overall decision is "aligned" only when every criterion is satisfied, "not_aligned" when at least one criterion is unsatisfied, and otherwise "inconclusive".
- Submit the report through the current Goal command contract as one report_json object. The backend validates criterion coverage and evidence shape before any lifecycle may consume the result.
- An aligned report is evidence for a lifecycle decision, not the lifecycle transition itself. Goal completion still requires update_goal; a loop guard will independently choose its control edge.
`)
}
