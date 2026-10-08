# Recovery ownership boundary — 2026-09-28

Nexus base `4c0c81597`, plus this batch. All Go commands use `GOWORK=off`.

Explicit Manager process recovery now requires an ownership callback. The real
macOS sidecar Guard verifies the original app directory and lock inode and holds
its mutex/file until the callback finishes. Concurrent Close cannot release flock
while recovery is mutating the original process record. The Manager checks that
its configured process directory lies under the same owned app root and matches
the original directory inode before native recovery.

## Verification

- `go test -race -count=1 -v -timeout=90s ./internal/infra/desktopinstance ./internal/runtime -run 'TestDesktopInstance|TestDesktopRecoveryOwnership|TestSandboxProcessRecovery'`: passed.
- Real flock test: a second instance is rejected during the callback and a concurrent
  Close waits until completion. Closed ownership does not invoke the callback.
- Real Guard + SQLite recovery adapter test: missing/closed/cross-root locks, replaced
  process directory and replaced lock inode never reach the injected native function;
  original prepared records remain intact. Restored original ownership permits only
  the exact prepared record to become aborted.
- Existing prepared/released/native-failure/active-client recovery tests pass;
  process recovery does not clear policy unknown records or replay terminal recovery.
- `go test -count=1 ./cmd/nexus-server -run '^TestDesktop'`, targeted vet of runtime
  and desktopinstance, and architecture check passed.

The concurrency fixture originally compared `/var` and `/private/var` path strings
and could block its own cleanup after a failed assertion. The final test compares
directory identities and always releases its callback before closing the Guard.
Final passing logs are archived; trailing whitespace normalized.

Native recovery is injected in the adapter test. Real process retirement evidence
remains in the earlier Bridge recovery tests; this batch proves ownership admission
and lifetime, not another native crash recovery or automatic App startup scan.
Older hosts that never acquired this lock, policy/scratch reconciliation, App default
supervision and release/platform acceptance remain separate. No Windows checks.
