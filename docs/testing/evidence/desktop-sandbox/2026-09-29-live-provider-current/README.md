# Current live third-party provider probe

- Date: 2026-09-29
- Nexus commit: `4546c484d`
- SDK commit: `169e5c31`
- Bridge commit: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- Runtime binaries: arm64 nxs SHA-256 `d88e520f9ecee1c7a980e4a2c39de66248ba9434be1b82d2fd45b9875a190ac`; Claude Code `2.1.273`
- Provider: third-party Anthropic-compatible endpoint selected from the main workspace `.env`; token output was redacted and no credential was archived.
- Command: `NEXUS_SANDBOX_TEST_BINARY=<nxs> NEXUS_SANDBOX_CLAUDE_BINARY=<claude> NEXUS_SANDBOX_LIVE_RUNTIME=both NEXUS_SANDBOX_LIVE_ENV_FILE=<workspace>/.env node scripts/desktop/check-live-sandbox.mjs`
- Result: exit 0; nxs and Claude subtests passed.

The probe exercised real model Write/Read/Bash, protected file read/write denial,
sidecar identity protection, Ruby child file-write denial, denied network access,
and Interrupt/Close cleanup with the fixture process observed exited. The raw
redacted output is in [live-provider.log](live-provider.log).

This is real provider/runtime isolation evidence. It does not prove graphical App
DM/Room/background acceptance, arbitrary MCP/provider network isolation, signed or
notarized packages, clean-host installation, Intel packaging, or release acceptance;
`releaseAccepted=false` remains required.
