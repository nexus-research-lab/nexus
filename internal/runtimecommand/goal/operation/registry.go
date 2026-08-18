// INPUT: Goal service 与 command context context。
// OUTPUT: 模型可见的完整 Goal 工具集合。
// POS: Goal command 操作注册入口。
package operation

import (
	runtimecommand "github.com/nexus-research-lab/nexus/internal/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
)

// BuildAll 汇集 Codex Goal 对齐的模型可见工具。
func BuildAll(svc contract.Service, sctx contract.Context) []runtimecommand.Operation {
	return []runtimecommand.Operation{
		getGoal(svc, sctx),
		createGoal(svc, sctx),
		retargetGoal(svc, sctx),
		auditObjectiveAlignment(svc, sctx),
		updateGoal(svc, sctx),
	}
}
