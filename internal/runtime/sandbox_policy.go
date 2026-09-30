// INPUT: Host-managed desktop sandbox marker and old/new approval modes.
// OUTPUT: A required runtime replacement when crossing the Full Access boundary.
// POS: Execution-policy transition guard before any hot permission update.
package runtime

import (
	"errors"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrDesktopSandboxPolicyChanged requires retiring the old process before new work.
// It is an expected transition, not an approval rejection or a request to replay work.
var ErrDesktopSandboxPolicyChanged = errors.New("desktop sandbox policy requires runtime replacement")

func desktopSandboxModeTransition(options bridge.Options, next sdkpermission.Mode) bool {
	return options.Env[protocol.NexusDesktopSandboxPolicyEnvName] == "1" &&
		(options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions) != (next == sdkpermission.ModeBypassPermissions)
}
