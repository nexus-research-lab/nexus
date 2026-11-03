// INPUT: Nexus runtime round identity, pinned nexus-cua Go client, and host permission context.
// OUTPUT: Owner-scoped Computer Use sessions, opaque target state, bounded observations, and mutation reconciliation.
// POS: The only service boundary for Nexus desktop execution; transport credentials, native handles, and sidecar lifecycle stay private.
//
// Package computeruse supervises one host-local nexus-cua sidecar epoch and
// keeps application/session/observation state bound to a physical Nexus round.
// Closing a round revokes its runtime session before waiting for active mutations;
// a session opened concurrently with close is immediately revoked on return.
// Supported runtimes return a post-action observation using the previous read
// options. Successful mutation evidence survives read failures; replay caches
// retain no screenshots or accessibility trees. User takeover remains latched
// until another explicit target approval, including after fresh observations.
// authority.go defines approved target scope and deadlines; launch.go owns
// launch discovery, approval and reconciliation without granting input authority.
package computeruse
