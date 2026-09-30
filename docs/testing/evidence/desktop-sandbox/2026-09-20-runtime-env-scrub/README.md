# Runtime inherited-environment scrub evidence (2026-09-20)

Nexus implementation commit: `a1eaa106f` (`:lock: Scrub provider and helper
secrets from runtime environment`). The working tree was the isolated
`codex/desktop-sandbox-isolated` checkout; no push and no main-worktree change
were made.

The change expands `internal/runtime/clientopts.scrubInheritedRuntimeEnv` to
clear known SDK bootstrap descriptors/fallbacks, Provider credentials and
custom headers, WebSearch/WebFetch helper credentials, TLS client material,
OTEL headers, SSH agent/command hooks, and Connector client secrets before the
Bridge transport inherits the host environment. Later host-resolved Provider
values are still projected explicitly by `BuildAgentClientOptions`.

## Commands and results

All commands ran from the Nexus worktree with exit code 0:

```text
GOWORK=off GOPROXY=off go test ./internal/runtime/clientopts -count=1
GOWORK=off GOPROXY=off go test -race ./internal/runtime/clientopts -count=1
GOWORK=off GOPROXY=off go test ./internal/runtime -count=1
GOWORK=off GOPROXY=off go vet ./internal/runtime/clientopts ./internal/runtime
```

The focused tests cover non-empty host secret/handle values becoming explicit
empty overrides and empty host variables not creating synthetic entries. The
existing client-options tests also confirm that the resolved session Provider
credential is projected after the scrub and cannot be replaced by task-scoped
environment input.

## Limits

This evidence closes known environment-source inheritance only. It does not
prove arbitrary secret-file access, already-open descriptor cleanup, external
MCP or authentication-helper process confinement, descendant environment
sanitization, network egress enforcement, Claude native sandbox behavior, or
Windows/Linux/macOS clean-host and signed-package acceptance. `releaseAccepted`
remains `false`.
