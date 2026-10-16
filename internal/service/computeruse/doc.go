// INPUT: Nexus runtime round identity, pinned nexus-cua Go client, and host permission context.
// OUTPUT: Owner-scoped Computer Use sessions, opaque target state, bounded observations, and mutation reconciliation.
// POS: The only service boundary for Nexus desktop execution; transport credentials, native handles, and sidecar lifecycle stay private.
//
// Package computeruse supervises one host-local nexus-cua sidecar epoch and
// keeps application/session/observation state bound to a physical Nexus round.
package computeruse
