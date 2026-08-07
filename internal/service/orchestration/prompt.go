// INPUT: 编译期稳定的最小 execution capability contract。
// OUTPUT: DM、Room、主智能体、普通 Agent 与 Goal continuation 共用的短系统提示段。
// POS: 常驻安全边界；详细选择方法按需加载 execution-orchestrator Skill。
package orchestration

import (
	_ "embed"
	"strings"
)

//go:embed prompt_policy.md
var stablePrompt string

// StablePrompt 返回去除外围空白后的稳定执行契约。
func StablePrompt() string {
	return strings.TrimSpace(stablePrompt)
}
