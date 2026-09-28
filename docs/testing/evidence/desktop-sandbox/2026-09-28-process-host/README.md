# macOS supervision Host database adapter

Status: component integration passed; production transport and release acceptance remain incomplete.

Bridge is pinned through the canonical Go module to `95b9616f8770ad2f31d1701b31aa80e17306dead` (`v0.1.34-0.20260928023432-95b9616f8770`). All Go commands used `GOWORK=off`; no Windows checks ran.

- `go test -race -count=1 ./internal/runtime -run '^TestSandboxProcessHost'`: real SQLite callback integration, lost prepare/register/release responses, once-only release, exact evidence, symlink rejection and long-path rejection. The opt-in native case skips in this command and runs separately below.
- `NEXUS_SUPERVISION_TEST_HELPER=/tmp/nexus-process-host-bootstrap go test -race -count=1 -json -timeout=90s ./internal/runtime -run '^TestSandboxProcessHostNativeLaunch$'`: independently built pinned helper, actual launchd job and task stdout, native collection retirement and exact SQLite terminal record. Passed without skip.
- `go test -count=1 ./internal/runtime`: runtime package regression passed.
- `CGO_ENABLED=0 go test -count=1 ./internal/runtime -run '^TestSandboxProcessHost'`: database/file adapter compiled and passed without cgo.
- `go vet ./internal/runtime` and architecture dependency check passed.

The short temporary root is a test fixture, not an approved product root. These tests do not establish App file-policy protection, default client transport integration, crash recovery, macOS 14 compatibility, long state-root support, signed distribution or clean-host acceptance. No unknown policy/lease records are cleared by this adapter. SDK and task side effects are not replayed.
