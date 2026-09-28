# Managed AutoDream lifecycle — 2026-09-28

Nexus base `c9021ac98`, plus changes committed with this evidence. Fixed Bridge
`v0.1.34-0.20260928032121-5e6db3dc4bef`. macOS arm64. All Go commands use `GOWORK=off`.

The host maintenance runner now uses the shared Manager. Startup, scratch ownership,
policy receipts, explicitly configured process supervision, shutdown and Agent
revocation no longer require an independent raw Bridge session.

## Verification

- `go test -race -count=1 -v ./internal/runtime -run TestManagedAutoDream`: success and scratch transfer; deliberately context-unresponsive control canceled by caller/shutdown/Agent revocation; cleanup failure fence; connection failure cleanup. All passed.
- `go test -race -count=1 ./internal/service/memorymaintenance`: passed.
- `go test -count=1 ./internal/runtime -run 'TestManager.*(Background|Revoke|Close)|TestManagedAutoDream'`: passed.
- Targeted vet of runtime, memorymaintenance and app; architecture gate: passed.
- Explicit `go test -race -count=1 -v ./internal/app -run '^TestAppManagedAutoDreamSupervisedNative$'`: passed without skips, with real fixed nxs and signed helper. Two control requests verify incrementing durable generation, exact scratch ID in the process record, reaped process and retired policy records, and deleted scratch directories.

The nxs binary is the fixed SDK `2d1fd0d6` baseline, SHA-256
`6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa`.
`NEXUS_SUPERVISION_TEST_HELPER` pointed to the ad-hoc signed helper from the pinned
Bridge module. Native tests use an isolated SQLite/state root and short `/tmp`
job-root fixture; this is not a production directory protection or long-path proof.

AutoDream consolidation was explicitly disabled in the native test. It proves
initialize/control/cleanup and durable lifecycle, not actual model consolidation.
No real model request, App UI, Windows validation or official Claude authentication
was performed. Default App supervisor setup, automatic recovery, macOS 14.0 support
and formal distribution remain open. Log trailing whitespace normalized.
