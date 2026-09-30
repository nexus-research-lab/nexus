# Manager process-supervision identity binding

Status: explicit Manager binding and ordered process registry passed; App default activation and release acceptance remain incomplete. Bridge remains pinned to `e8787a420df14cd7725f0744ef5b674ea433f3ba`.

All Go commands used `GOWORK=off` on macOS arm64. No Windows native validation or cross-compilation ran.

- Native `TestSandboxProcessSupervisorNativeStartup` passed under race instrumentation, with the pinned real bootstrap helper, actual launchd and SQLite. A controlled client factory captures the Manager-issued configuration; each of the four purpose stages then launches a real command and reaches exact native retirement in the same generation. This does not claim a real model or full App session.
- Runtime binding race tests passed: frozen owner/session/generation before factory, separate purposes, restart generation floor, unresolved probe blocking before factory, reconfiguration preserving supervision, untrusted replacement and unknown-purpose rejection. A separate final race case verifies warm-client replacement uses the next live generation.
- SQLite purpose/migration race tests passed: ordered same-generation launches, one active slot, no stage replay, original migration-144 intent JSON preserved, malformed order rejected, and rollback refusing multiple launches without losing rows or changing migration version. PostgreSQL migration was reviewed, not run against a PostgreSQL server.
- Runtime and sandbox-storage package regressions passed. No-cgo supervisor tests, targeted vet and the architecture gate passed.

Commands:

```sh
GOWORK=off go test -race -count=1 ./internal/storage/sandbox -run '^TestProcessPurpose'
GOWORK=off go test -race -count=1 ./internal/runtime -run '^TestSandboxProcess(Supervisor|Host|Intent|Terminal|Unverifiable)'
GOWORK=off go test -race -count=1 ./internal/runtime -run '^TestSandboxProcessSupervisorReplacementUsesNextLiveGeneration$'
NEXUS_SUPERVISION_TEST_HELPER=/tmp/nexus-supervised-transport-bootstrap GOWORK=off go test -race -json -count=1 -timeout=90s ./internal/runtime -run '^TestSandboxProcessSupervisorNativeStartup$'
GOWORK=off go test -count=1 ./internal/runtime ./internal/storage/sandbox
CGO_ENABLED=0 GOWORK=off go test -count=1 ./internal/runtime -run '^TestSandboxProcessSupervisor'
GOWORK=off go vet ./internal/runtime ./internal/storage/sandbox
GOWORK=off make check-architecture
```

Fixture roots and helper hashes are test inputs, not product path protection or signed distribution. App initialization still must supply the trusted root/helper digest, attach the actual scratch lease, and implement restart reconciliation and packaging. Long roots, supported older macOS versions and full release acceptance remain open. No unknown task effects are replayed.
