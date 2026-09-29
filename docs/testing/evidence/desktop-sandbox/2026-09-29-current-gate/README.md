# Current macOS host gate

- Date: 2026-09-29
- Nexus commit: `678eb0ee5`
- SDK commit: `169e5c31`
- Bridge commit: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs: arm64 build from the SDK worktree
- nxs SHA-256: `d88e520f9ecee1c7a980e4a2c39de66248ba9434be1b82d2fd45b9875a190ac`
- Command: `NEXUS_SANDBOX_TEST_BINARY=<absolute nxs> make check-desktop-sandbox`
- Result: passed; scope is `host-integration-only`.

The gate covered the configured host policy, lifecycle, cancellation, settings recovery,
application shutdown and MCP round-trip checks. This evidence does not prove Developer ID
signing, notarization, clean-host installation, Intel packaging, full graphical App acceptance,
or release acceptance; `releaseAccepted=false` remains required.
