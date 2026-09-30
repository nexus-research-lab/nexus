# macOS sidecar state-root ownership

Status: kernel instance lock wired into the macOS desktop server entrypoint; automatic process recovery and release acceptance remain incomplete.

The Go sidecar acquires `app/sidecar.lock` after environment loading and before layout migration, config/credential resolution, database migration or service startup. Its defer releases the guard after the server close defer. The file is never removed and carries no PID. The macOS shell's separate single-instance lock remains distinct.

All Go commands used `GOWORK=off` on macOS arm64. Verified:

- `go test -race -json -count=1 -timeout=60s ./internal/infra/desktopinstance`: same-root exclusion, inode retention across Close/reacquire, CLOEXEC, symlink/hardlink rejection, file/directory replacement detection, and actual child-host exit releasing ownership. No historical PID is signaled.
- `go test -count=1 ./cmd/nexus-server`: entrypoint package regression passed.
- `go test -race -count=1 ./cmd/nexus-server -run '^TestDesktopInstanceLock'`: lock acquired before legacy layout migration; database/session fixture bytes and lock identity preserved, second sidecar still rejected.
- `CGO_ENABLED=0 go test -count=1 ./internal/infra/desktopinstance`: native lock tests passed without cgo.
- Targeted vet and architecture gate passed.

The lock coordinates only sidecars adopting this protocol. It does not prove that an older uncoordinated host exited, replace the shell's existing orphan handling, or establish task descendant retirement. The current App still does not automatically configure the new supervisor or invoke recovery under this guard. Helper packaging, legacy-host handling, Guard-to-recovery wiring, scratch/policy reconciliation, supported-version and signed clean-host acceptance remain outstanding. No Windows native tests or cross-compilation ran.
