# Original process-registration recovery

Status: explicit recovery components passed; automatic App crash recovery and release acceptance remain incomplete.

Bridge: `5e6db3dc4bef9c0b0a54e116d5bed68386a3d0a2`. All Go commands used `GOWORK=off` on macOS arm64; no Windows native validation or cross-compilation ran.

- Native `TestRecoveryNativeHostExit` under race instrumentation starts a real launchd task from a separate host process. That host exits without Close/Finish; the parent confirms that the exact job still has a running root, reads the original fixture registration, and recovers the original collection. Repeated recovery succeeds without another task launch. The fixture uses files, not the Nexus DB.
- Bridge recovery race tests check validation before side effects, each failed native/store phase, cancellation, exact evidence binding and changed-boot behavior (the latter is simulated, not an actual reboot). No-cgo returns unavailable without Finish.
- Nexus adapter race tests use real SQLite records with injected native outcomes. They cover prepared/released retirement, terminal rereads without re-invoking native recovery, active-client rejection, cross-owner rejection, native failure retaining released state, and successful process retirement preserving policy unknown.
- Nexus runtime package regression, targeted vet and architecture check passed.

Commands:

```sh
NEXUS_SUPERVISION_TEST_HELPER=/tmp/nexus-supervised-transport-bootstrap GOWORK=off go test -race -json -count=1 -timeout=90s ./supervision -run '^TestRecoveryNativeHostExit$'
GOWORK=off go test -race -count=1 ./supervision
CGO_ENABLED=0 GOWORK=off go test -count=1 ./supervision
GOWORK=off go test -race -count=1 ./internal/runtime -run '^TestSandboxProcessRecovery'
GOWORK=off go test -count=1 ./internal/runtime
```

The original-host exit test does not establish full App recovery, actual reboot handling, lifecycle exclusion between App instances, scratch/policy reconciliation, supported older macOS versions or signed packaging. The explicit Nexus recovery API requires callers to own the cross-process App instance lock and confirm that the old host has exited. It does not acquire that external lock or automatically scan/recover on App startup. It never replays task commands or clears unrelated state.
