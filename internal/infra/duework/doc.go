// Package duework provides the process-local wake and exact-deadline driver
// shared by durable background coordinators.
//
// L2 | Parent: internal/infra (L1 in AGENTS.md)
//
// Members:
//   - loop.go: coalesced wake, one-shot deadline timer, low-frequency audit and
//     bounded error retry lifecycle.
//
// The package is deliberately unaware of databases and business states. A
// durable domain row plus claim/CAS remains the source of truth; notifications
// only reduce latency and audits recover missed notifications.
//
// [PROTOCOL]: behavior changes require checking domain coordinators and AGENTS.md.
package duework
