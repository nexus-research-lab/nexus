// INPUT: 编译期稳定 execution contract。
// OUTPUT: 可由 DM、Room、主智能体、普通 Agent 与 Goal continuation 共用的系统提示段。
// POS: Execution Orchestration 模型决策规则的唯一文本真相源。
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
