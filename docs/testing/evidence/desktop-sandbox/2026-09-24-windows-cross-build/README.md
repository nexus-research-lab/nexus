# Windows sandbox cross-architecture gate evidence (2026-09-24)

This record covers the Nexus working tree at commit `e9a15522db53cafe8f653ce3ef43871acd34dfed`.
The tree was dirty and the run was performed on macOS arm64, so this is build and
contract evidence rather than native Windows acceptance. The raw gate report is
kept in [`report.json`](./report.json), SHA-256
`3017be29850910fb77a1f83f4d554c274e96cdd89ebf0c5cb26cdbbbd52722af`.

## Command and result

```text
make check-desktop-sandbox-windows
```

The command exited 0. It verified the Windows installer single-instance and
per-user contract, then built the following for both `windows/amd64` and
`windows/arm64` with `GOWORK=off` and `CGO_ENABLED=0`:

- `internal/runtime` test executable;
- `internal/runtime/clientopts` test executable;
- `internal/infra/confinedfs` test executable;
- `cmd/nexus-server`, `cmd/nexusctl`, and `cmd/nexuscfg`.

The generated report was `scope=windows-cross-build`, `passed=true`, and
`releaseAccepted=false`.

The pinned SDK component also compiled from commit
`9956def130da33af47accf799a9c27c16a551104` on the same host:

```text
GOWORK=off CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go test -mod=readonly -c ./internal/tool/builtin/bash/sandboxexec
GOWORK=off CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
  go test -mod=readonly -c ./internal/tool/builtin/bash/sandboxexec
GOWORK=off go test ./internal/tool/builtin/bash/sandboxexec -count=1
GOWORK=off go vet ./internal/tool/builtin/bash/sandboxexec
```

Both Windows test compilations, the macOS-hosted SDK package tests and vet
exited 0. They cover component/build compatibility only; the SDK still reports
native Windows execution unsupported and does not turn these results into a
runner acceptance claim.

## Identity change

Scratch markers now optionally carry `process_start_time_unix_nano`. The
Windows implementation queries `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)`
and `GetProcessTimes`; a live PID with a different creation time is treated as
PID reuse, while access or query failures remain unknown and are retained.
Older markers and platforms without a safe creation-time probe use the existing
conservative liveness behavior. `cleanup_unknown` is never overridden by this
check.

## Limits

- This host did not run the Windows-specific identity tests, so there is no
  Windows 11 amd64/arm64 native log yet.
- The SDK's native Windows `PrepareExecution` and `InspectBackend` remain
  fail-closed. Existing token, Job Object, private desktop, pipe, and ACL tests
  are component evidence only; they are not a complete Nexus → Bridge → nxs
  restricted command-chain proof.
- P3 compatibility/isolation combinations, real cancellation and descendant
  termination, account/ACL/network provisioning, signed installation,
  upgrade/rollback, clean-host behavior, and release acceptance remain open.
