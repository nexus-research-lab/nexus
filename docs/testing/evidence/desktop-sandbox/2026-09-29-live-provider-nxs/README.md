# 2026-09-29 live third-party provider nxs probe

The real third-party Anthropic-compatible provider probe passed for nxs only using
SDK `7090d9c58f37925212d50c0d6a41299ee0ced643` and binary SHA-256
`a9e9f632a89082a05113615f6e1d45c7fc83a99f14ac55b53d425e67709f3c06` on macOS 27.0
arm64. The run took 69.45 seconds and exited 0.

It exercised real model tool calls for native Write, Read and Bash; denied protected
file read/write, Ruby child file IO, sidecar identity file access and network access;
and Interrupt/Close cleanup with the fixture process independently observed exited.
The runner read only token/base URL/model from the ignored `.env` and redacted token
output. No database, owner state or credentials were archived.

```text
NEXUS_SANDBOX_TEST_BINARY=/tmp/nexus-live/nxs \
NEXUS_SANDBOX_LIVE_RUNTIME=nxs \
NEXUS_SANDBOX_LIVE_ENV_FILE=/Users/berhand/program/Work/Nexus/worktrees/nexus/desktop-sandbox/.env \
node scripts/desktop/check-live-sandbox.mjs
```

This is a real nxs/provider isolation probe, not graphical App DM/Room/automation
acceptance, signed-package acceptance, clean-host/Intel validation, or release
acceptance; `releaseAccepted=false`.
