// Package queueadmission persists one-time, payload-bound trust for direct user
// input that crosses an Agent-writable queue before a runtime starts.
//
// L2 | parent: internal/storage (L1 in AGENTS.md)
//
// Members:
//   - model.go: canonical queue/payload binding plus opaque host-auth principal identity.
//   - repository.go: idempotent admission, principal-bound single-use claim, release, consume, and revoke.
//
// Exposed: Admission, Binding, PrincipalBinding, Claim, NewBinding, Repository, NewRepository.
//
// [PROTOCOL]: update this header when the package contract changes.
package queueadmission
