# Restricted rewind executor gate

- Date: 2026-09-29
- Nexus source: `b5a66bf6d`
- SDK source: `ac2c41f9`
- Bridge source: `c251a8d`
- Branch: `codex/desktop-sandbox-approvals`
- Scope: `host-integration-only`
- Result: passed

The nxs binary was built and consumed in the same process environment to avoid
transient temporary-path cleanup. The gate covered host policy, lifecycle,
cancellation, settings recovery, application shutdown and MCP round-trip after
restricted rewind was routed through the file executor. This remains separate
from graphical App, macOS 14.0, signing/notarization, clean-host and release
acceptance; `releaseAccepted=false` remains required.
