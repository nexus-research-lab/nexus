# Supervised scratch lease binding — 2026-09-28

Nexus base: `32d8deaa5`, plus changes committed with this evidence. Fixed Bridge:
`v0.1.34-0.20260928032121-5e6db3dc4bef`.

The real signed helper `/tmp/nexus-packaged-bootstrap-signed` was supplied through
`NEXUS_SUPERVISION_TEST_HELPER`. All Go commands used `GOWORK=off`.

- Native race run: `go test -race -count=1 -v ./internal/runtime -run 'TestSandboxProcessSupervisor|TestSupervisedLease|TestAgentClientBindSandboxLease'`. No skips. Real launchd helper tasks for Claude admission probes/runtime and nxs version/runtime all retired; nxs snapshots retained the exact acquired lease ID.
- Factory-time SQLite test checks persisted lease identity before ownership transfer; mismatched handle transfer preserves caller ownership. Missing/foreign/policy-mismatched/released/cleanup-unknown resources are rejected.
- After adding warm-client admission, final targeted tests passed for scratch binding (including warm missing-lease rejection), invalid lease identity and replacement generation.
- DM/Room targeted tests: `go test -count=1 ./internal/service/dm ./internal/service/room/realtime -run 'Sandbox|Scratch|RuntimeClient|RuntimeStartup'` passed.
- Targeted vet of runtime/DM/Room and `make check-architecture` passed. Log trailing whitespace normalized.

These are explicit supervisor tests and service regressions. Default App supervisor
setup, AutoDream's separate NewSession path, automatic process/policy/lease recovery,
macOS 14.0 compatibility and release acceptance remain open. No Windows checks.
