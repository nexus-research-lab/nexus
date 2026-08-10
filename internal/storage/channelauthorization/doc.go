// Package channelauthorization persists scope-bound Channel authorization
// flows and immutable, secret-free completion records.
//
// L2 | parent: internal/storage (L1 in AGENTS.md)
//
// Members:
//   - model.go: trusted flow binding, terminal states, and audit model.
//   - repository.go: active-flow uniqueness/listing, commit-generation lookup,
//     CAS transitions, restart invalidation, encrypted ephemeral payload scrubbing,
//     and immutable completion audit.
//
// Exposed: Binding, Flow, TerminalUpdate, Audit, Repository, NewRepository.
//
// [PROTOCOL]: update this header when the package contract changes.
package channelauthorization
