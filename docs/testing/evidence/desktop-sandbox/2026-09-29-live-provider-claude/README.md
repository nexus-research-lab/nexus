# 2026-09-29 live third-party provider Claude probe

The real third-party Anthropic-compatible provider probe passed for Claude Code only
on macOS 27.0 arm64. Claude Code was `/Users/berhand/.local/bin/claude` version
`2.1.273`; the test used the current Nexus branch and the selected `.env` token,
base URL and model only. The run took 71.34 seconds and exited 0.

The real model exercised Write, Read and Bash; native file deny, Ruby child file IO,
Sidecar identity file access and network deny all passed; Interrupt/Close independently
observed the fixture process exit. Output was redacted and no state/database/credential
files were archived.

```text
NEXUS_SANDBOX_CLAUDE_BINARY=/Users/berhand/.local/bin/claude \
NEXUS_SANDBOX_LIVE_RUNTIME=claude \
NEXUS_SANDBOX_LIVE_ENV_FILE=/Users/berhand/program/Work/Nexus/worktrees/nexus/desktop-sandbox/.env \
node scripts/desktop/check-live-sandbox.mjs
```

This proves Claude runtime isolation against this third-party gateway only. It does not
prove graphical App DM/Room/automation flows, persistent approval UI, signed packages,
clean-host/Intel installation, or release acceptance; `releaseAccepted=false`.
