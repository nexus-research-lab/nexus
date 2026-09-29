# macOS package and smoke evidence

- Date: 2026-09-29
- Nexus branch: `codex/desktop-sandbox-approvals`
- App architecture: arm64
- Package: ad-hoc signed zip built from the current Nexus worktree.
- Runtime: current SDK worktree `make build-nxs NXS_GOOS=darwin NXS_GOARCH=arm64`, supplied through `NEXUS_NXS_COMMAND_PATH`; this run intentionally did not claim bundled-runtime packaging because the GitHub `nxs-stable` manifest download timed out.
- State root: fresh isolated `NEXUS_DESKTOP_STATE_ROOT`.
- Result: `scripts/desktop/smoke-macos-app.sh` passed, including sidecar startup, migration to schema 149, launcher ready, route navigation and clean exit.

This is a local arm64 ad-hoc smoke result. It does not prove Developer ID signing, notarization, clean-host installation, Intel packaging, upgrade/rollback, or release acceptance.

- Runtime upgrade gate: `scripts/desktop/check-runtime-upgrade.mjs` passed with the existing `nexus-agent-sdk-go-rewrite-fix` arm64 binary as previous and the current SDK worktree candidate. All required `before_upgrade`, `after_upgrade`, and `after_rollback` cases passed. The gate remains scoped to runtime data compatibility and keeps `releaseAccepted=false`.
- Host baseline rerun: `NEXUS_SANDBOX_TEST_BINARY=<current arm64 nxs> make check-desktop-sandbox` passed the configured host integration checks, including stdio MCP roundtrip, lifecycle, shutdown, policy, settings recovery and cancellation. The command reports host integration scope only; native release acceptance remains separate.
- SDK target tests: `go test ./internal/session ./internal/agent/runtime ./internal/mcp/client` passed. Restricted transcript loading remains on the explicit `TranscriptFiles` port; legacy direct file access is retained only for Full Access compatibility.
