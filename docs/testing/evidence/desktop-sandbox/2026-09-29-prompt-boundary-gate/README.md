# macOS host gate after restricted prompt IO boundary

- Date: 2026-09-29
- Nexus commit: `8b7512f84`
- SDK commit: `b3064c83`
- Bridge commit: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs SHA-256: `71940cdc43446dcb1b7c56e95f1821eab8fb8b52a25682f062b9172d60f19db7`
- Command: `NEXUS_SANDBOX_TEST_BINARY=/tmp/nxs-prompt-boundary make check-desktop-sandbox`
- Result: passed; scope is `host-integration-only`.

This run revalidated host policy, lifecycle, cancellation, settings recovery,
application shutdown and MCP round-trip checks after the SDK stopped spawning
unsandboxed Git and OS-version helpers during restricted prompt construction.
It does not prove graphical App acceptance, signing, notarization, clean-host
installation, Intel packaging or release acceptance; `releaseAccepted=false`
remains required.
