# Proven multi-document settings rollback evidence (2026-09-18)

This archive records the SDK follow-up after the cross-process lock batch.
SDK commit `9d60e166` now rolls back already-applied settings documents in
reverse order when a later write or post-write verification fails and the
current files still match the attempted update. A document created during the
failed update is removed with the same physical-directory checks. If the
current state cannot be proven, the existing unknown fence is retained.

The fixed Bridge remains `8a4576ba97ece60e0485f2bfbb0bce53e5b89502` through
module `v0.1.34-0.20260918053632-8a4576ba97ec`
(`h1:nYthJgS+xL7KZayRvMhy086O/xXtJ2KcqGjB/xMPwyo=`). The nxs binary built
from SDK `9d60e166` has SHA-256
`374a022e84a1dd081c2c9e2b56474dcc61dbfd4868b70f9fbeaf05de8ff49330`.

Target and race tests pass, including explicit rollback coverage for newly
created and existing documents. The Nexus
`make check-desktop-sandbox` gate passes on the current macOS arm64 host with
the fixed Bridge and the new nxs binary; no model request was sent.

Compressed command output, the baseline report and manifest are kept here.
`releaseAccepted=false` remains: power-loss all-or-nothing recovery, durable
SDK request receipts, inspect/reconcile UI, Provider/auxiliary process and
network isolation, descendant cleanup, native Windows/Linux acceptance,
authenticated Claude sessions and release packaging are still open.

Re-run:

```sh
GOWORK=off GOPROXY=off go test ./internal/config/settings ./cmd/nxs ./internal/agent/runtime ./internal/tool/builtin/config
GOWORK=off GOPROXY=off go test -race ./internal/config/settings ./internal/agent/runtime
GOWORK=off GOPROXY=off go build -o /tmp/nxs-settings-rollback ./cmd/nxs
NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-settings-rollback make check-desktop-sandbox
```
