// Package sandboxresources owns desktop runtime scratch directories.
// The host creates and removes each lease; SDK/Bridge only negotiates the
// resulting versioned policy and never performs lifecycle cleanup.
//
// L2 | parent: internal/runtime (L1 in AGENTS.md)
//
// Members:
//   - scratch.go: owner/session scoped directory creation, policy validation,
//     active-scope reuse, idempotent release and a guarded cleanup registry.
//   - scratch_test.go: host lease and close-fence regression tests.
//
// Exposed interfaces: Input, Lease, Acquire, ReleasePath.
package sandboxresources
