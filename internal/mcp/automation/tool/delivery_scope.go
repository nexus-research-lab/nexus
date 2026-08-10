// INPUT: 服务端签发的 Automation 会话身份、任务 Agent 与解析后的投递目标。
// OUTPUT: owner-main 放行，普通 Agent/Room 成员仅保留 self/current-context 投递。
// POS: nexus_automation create/update 在写入前的投递目标授权边界。
package tool

import (
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
)

func requireConversationDeliveryScope(
	sctx contract.ServerContext,
	taskAgentID string,
	target automationdomain.DeliveryTarget,
) error {
	if hasMainAgentScopeAuthority(sctx) {
		return nil
	}
	return automationdomain.ValidateSelfScopedDeliveryTarget(
		taskAgentID,
		sctx.CurrentSessionKey,
		target,
	)
}
