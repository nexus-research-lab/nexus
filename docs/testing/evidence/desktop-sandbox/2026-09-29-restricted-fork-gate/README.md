# Restricted fork file-executor gate

- Date: 2026-09-29
- Nexus source: `9bd34f497`
- SDK source: `069cfa86`
- Bridge source: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs SHA-256: `c2e2f656348aed7356cfe057d8de2550061385dc38b81b9fb85c44173204b634`
- Command: `GOWORK=off GOPROXY=off go build -o /private/tmp/nxs-restricted-fork ./cmd/nxs`; `NEXUS_SANDBOX_TEST_BINARY=/private/tmp/nxs-restricted-fork make check-desktop-sandbox`
- Result: passed (`host-integration-only`)

The SDK restricted fork path now reads the canonical transcript through the
captured file executor, publishes the target transcript with exclusive create,
clones plans and artifacts through the same executor, rewrites artifact session
IDs atomically, and rejects symlink artifacts. Full Access retains the legacy
catalog and filesystem path. This gate does not prove graphical App, macOS 14.0,
signing/notarization, clean-host or production release acceptance;
`releaseAccepted=false` remains required.
