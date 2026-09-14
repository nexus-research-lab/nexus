# Desktop sandbox integration (experimental)

This document describes implemented host wiring, not acceptance of the full App
sandbox. Remaining design and delivery work is tracked in
[the exploration](../explorations/desktop-sandbox/README.md).

## Activation and scope

`NEXUS_DESKTOP_SANDBOX_ENABLED=true` is an explicit host rollout switch, defaulting
to false. It is effective only with `NEXUS_APP_MODE=desktop` on macOS or Windows.
Server deployments retain their existing runtime identity/isolation policy even
if the flag is present. Agent settings cannot set the internal host policy marker.

DM, Room and background memory maintenance use the same client-options builder.
For restricted approval modes, it requires nxs `required_sandbox_v1` negotiation;
an unsupported runtime fails instead of silently accepting an unenforced policy.
Negotiation is not proof that the current platform has a usable backend: Windows
native command execution is still incomplete and required commands currently fail
closed there. This wiring is not yet enabled by desktop packaging defaults.

The host passes Skill directories as read resources and user-mounted directories
as explicit sandbox write grants. The SDK additionally grants its stable workspace
and compatibility paths under its mandatory execution policy. Project settings
cannot expand that mandatory policy. New outbound shell proxy destinations are
denied unless permitted by the execution policy; network approval integration is
still outstanding. Explicit command escape continues through the SDK's independent
sandbox-bypass approval boundary, including auto-review where supported.

Mandatory SDK temporary-directory preparation uses a workspace directory handle
before the command sandbox starts, so descendant symlink replacement cannot redirect
the host's directory creation outside that root. Unsupported native backends reject
before temporary-directory or proxy allocation. These preparation guarantees do
not establish confinement for every other host-side filesystem operation.

Sandbox escape uses the typed `permission_boundary=sandbox_escape` classification.
Nexus shows an explicit outside-sandbox explanation and exposes only one-time
approval. Persistent rules supplied in a response are rejected by both Nexus and
the SDK; IM cannot persist a grant when no scope suggestion exists. The SDK also
rejects an allow response that adds escape or changes its reviewed JSON input while
remaining outside the sandbox. Ordinary tool input edits and returning an action
inside the sandbox retain their existing behavior. Unknown boundary classifications
are rejected before a pending approval is created.

This policy currently confines shell execution. It does not establish an OS boundary
around every in-process file tool, MCP server, Connector or the desktop UI. Those
tools retain their existing authorization; complete App sandbox coverage remains
unfinished. The feature must not be represented as fully accepted App isolation.

## Approval modes and runtime replacement

The host does not change the selected approval mode to enable sandboxing. A fresh
Full Access (`bypassPermissions`) runtime receives no additional host sandbox
policy; existing legacy settings and OS permissions retain their prior semantics.
This does not grant OS administrator privileges or override domain authorization.

For host-managed desktop policy, a live change crossing into or out of Full Access
retires the old client before returning the transition signal. DM closes the old
session; Room cancels the exact slot and its pending approval requests, retires the
client, and waits for cleanup. A confirmed close is an expected mode transition;
cleanup failure is reported. A subsequent request constructs fresh options and
the existing process-policy fingerprint requires replacement of the old runtime.
HTTP updates process both DM and Room even if one domain fails; their errors are
reported together instead of leaving the later domain on its old policy.

Changing between restricted approval modes keeps the sandbox boundary and uses
the existing permission-mode update path. This document does not claim complete
approval-revision invalidation across every host callback yet.
For SDK manual/automatic permission callbacks, effective rule/mode changes cancel
the pending policy epoch and invalidate even a late allow response. Identical
refreshes preserve the pending epoch. A cancelled request cannot create a fresh
Nexus pending prompt. The nxs proxy preserves human-only requirements and review
evidence; the reviewer treats sandbox escape as additional execution authority.

Mode changes do not resubmit a prompt or repeat a tool invocation. Stopping a
process does not roll back prior side effects; an unknown or partial command result
must not be interpreted as permission to replay it. Bridge cleanup waits for
transport exit, not merely stream closure. This confirms the runtime main process;
full descendant cleanup needs the remaining platform backend integration.
