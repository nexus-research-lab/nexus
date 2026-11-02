// INPUT: Authoritative Execution identity and a stable recovery receipt key.
// OUTPUT: Scope-specific durable coordination input.
// POS: App-only routing between orchestration and DM/Room services.
package execution

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type recoveryQueue interface {
	QueueExecutionRecovery(context.Context, protocol.Execution, string) error
}
type RecoveryConsumer struct{ DM, Room recoveryQueue }

func (c RecoveryConsumer) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	if e.ScopeKind == protocol.ExecutionScopeRoom {
		return c.Room.QueueExecutionRecovery(ctx, e, key)
	}
	return c.DM.QueueExecutionRecovery(ctx, e, key)
}
