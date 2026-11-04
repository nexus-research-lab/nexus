// INPUT: Nexus runtime round identity, pinned nexus-cua Go client, and host permission context.
// OUTPUT: Owner-scoped Computer Use sessions, opaque target state, bounded observations, and mutation reconciliation.
// POS: The only service boundary for Nexus desktop execution; transport credentials, native handles, and sidecar lifecycle stay private.
//
// Package computeruse supervises one host-local nexus-cua sidecar epoch and
// keeps application/session/observation state bound to a physical Nexus round.
// A selected target without a successful observation remains session-active;
// actions report missing observation without asking for a new target grant.
// Omitted window selectors reuse the exact current window while it exists;
// a missing window is replaced automatically only when there is one candidate.
// Explicit observations expose preceding discovery state without inferring
// minimization from thumbnail bounds; post-action reads do not reuse that state.
// Closing a round revokes its runtime session before waiting for active mutations;
// a session opened concurrently with close is immediately revoked on return.
// Installation status exposes required-version mismatches for explicit component upgrades.
// Supported runtimes return a post-action observation using the previous read
// options. Successful mutation evidence survives read failures; replay caches
// retain no screenshots or accessibility trees. User takeover remains latched
// until another explicit target approval, including after fresh observations.
// Running and launchable discovery accept name/id queries without selecting or
// granting authority; filtered launch catalogs retain only returned opaque refs.
// authority.go defines approved target scope and deadlines; launch.go owns
// launch discovery, approval and reconciliation without granting input authority.
// foreground*.go transfer Windows OS foreground eligibility through the native
// host's private pipe only for its live owned CUA child, after mutation approval.
// This best-effort handoff neither changes grants nor bypasses native readback.
// Windows child startup suppresses console creation so the sidecar cannot take
// foreground ownership away from the approving desktop host.
// installer.go verifies platform-specific release assets and executable names,
// preserves identical installed versions, and persists bundled-runtime enablement.
// Screenshot delivery normalizes Windows verbatim paths and opens each image
// beneath a pinned confinedfs root with bounded reads.
package computeruse
