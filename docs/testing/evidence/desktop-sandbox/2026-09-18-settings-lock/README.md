# Settings writes cross-process lock evidence (2026-09-18)

This archive records the settings-writes follow-up after the Notebook batch.
The SDK is fixed at `ce136cfe` and adds a stable per-physical-root lock,
context-cancellable acquisition, deterministic multi-root lock ordering,
pre-write snapshot revalidation, and directory-entry syncing after atomic
replacement. Bridge remains `8a4576ba97ece60e0485f2bfbb0bce53e5b89502` and
Nexus remains `223dd495a915842a7676ec7b5f95e852670203da`.

The fixed local Bridge module is
`v0.1.34-0.20260918053632-8a4576ba97ec` with checksum
`h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`. The nxs binary was built
from the SDK commit with `GOWORK=off GOPROXY=off`; its SHA-256 is
`3ec4aeb09208733c74a135f923f04fc0e89269f94b49530f1dc85d0779671afb`.

The SDK target tests and race tests passed. The Nexus
`make check-desktop-sandbox` gate passed on macOS arm64 with Go 1.27.1,
including the fixed Bridge module, host policy/lifecycle tests and the real
Nexus → Bridge → nxs negotiation path. No model request was sent.

This batch closes cross-process serialization evidence only. It does not claim
multi-document power-loss all-or-nothing, durable SDK request/approval/revision
receipts, restart inspect/reconcile UI, Provider secret-file/handle isolation,
external MCP confinement, network enforcement, descendant cleanup, native
Windows/Linux acceptance, Claude authenticated-session acceptance, or release
readiness. `releaseAccepted=false`; all commits remain local and were not
pushed.

Re-run from the corresponding worktrees:

```sh
GOWORK=off GOPROXY=off go build -o /tmp/nxs-settings-lock ./cmd/nxs
GOWORK=off GOPROXY=off go test ./cmd/nxs ./internal/config/settings ./internal/agent/runtime ./internal/tool/builtin/config
GOWORK=off GOPROXY=off go test -race ./internal/config/settings ./internal/agent/runtime
NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-settings-lock make check-desktop-sandbox
```
