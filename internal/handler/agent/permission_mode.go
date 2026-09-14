// INPUT: Persisted Agent permission mode and all active runtime domains.
// OUTPUT: Every domain is updated or retired, with combined failure reporting.
// POS: HTTP permission updates cannot leave Room on the old policy after a DM failure.
package agent

import (
	"context"
	"errors"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func syncAgentPermissionMode(ctx context.Context, agentID string, mode sdkpermission.Mode, setters ...agentPermissionModeSetter) error {
	var failures []error
	for _, setter := range setters {
		if err := setter.SetPermissionModeForAgent(ctx, agentID, mode); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
