// Package channelauthorization provides an owner-main private-DM-only,
// persistent conversational Channel authorization flow.
//
// L2 | parent: internal/service (L1 in AGENTS.md)
//
// Members:
//   - model.go: redacted model view and out-of-band human presentation contract.
//   - service.go / boundary.go: dependencies, lifecycle fence, process generation, dynamic
//     configuration authority revalidation, exact human route binding, and
//     same-principal private-DM recovery across active rounds.
//   - flow.go / synchronize.go: start/status/cancel/code-prompt, human-only code
//     submission, TTL, restart invalidation, version CAS, and completion audit.
//   - commit_guard.go / close.go: fresh pre-commit human/session lease plus
//     exact login cancellation, goroutine drain, and audited graceful shutdown.
//   - payload.go: key-identified active/legacy keyring encryption for ephemeral runtime and QR/device material.
//
// Exposed: Service, NewService, Actor, StartInput, View, Completion,
// HumanPresenter, HumanPresentation, HumanVerificationCodeSubmission.
//
// [PROTOCOL]: update this header when the package contract changes.
package channelauthorization
