# Live third-party nxs probe after restricted prompt IO boundary

- Date: 2026-09-29
- Nexus source used by probe: `c576a6310`
- SDK source: `b3064c83`
- Bridge source: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- nxs SHA-256: `71940cdc43446dcb1b7c56e95f1821eab8fb8b52a25682f062b9172d60f19db7`
- Provider: third-party Anthropic-compatible endpoint from the workspace `.env`
- Command: `NEXUS_SANDBOX_TEST_BINARY=/private/tmp/nxs-prompt-boundary NEXUS_SANDBOX_LIVE_RUNTIME=nxs NEXUS_SANDBOX_LIVE_ENV_FILE=<workspace>/.env node scripts/desktop/check-live-sandbox.mjs`
- Result: passed; nxs real model Write/Read/Bash, protected file and sidecar identity denial, Ruby child-process denial, network denial, and Interrupt/Close cleanup.

This is a live nxs runtime probe. It does not prove graphical App DM/Room/background UI,
macOS 14.0 compatibility, arbitrary detached descendants, signing/notarization, clean-host
installation or release acceptance; `releaseAccepted=false` remains required.
