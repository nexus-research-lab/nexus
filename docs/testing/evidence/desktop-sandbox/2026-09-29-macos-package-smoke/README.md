# macOS package and smoke evidence

- Date: 2026-09-29
- Nexus branch: `codex/desktop-sandbox-approvals`
- App architecture: arm64
- Package: ad-hoc signed zip built from the current Nexus worktree.
- Runtime: current SDK worktree `make build-nxs NXS_GOOS=darwin NXS_GOARCH=arm64`, supplied through `NEXUS_NXS_COMMAND_PATH`; this run intentionally did not claim bundled-runtime packaging because the GitHub `nxs-stable` manifest download timed out.
- State root: fresh isolated `NEXUS_DESKTOP_STATE_ROOT`.
- Result: `scripts/desktop/smoke-macos-app.sh` passed, including sidecar startup, migration to schema 149, launcher ready, route navigation and clean exit.

This is a local arm64 ad-hoc smoke result. It does not prove Developer ID signing, notarization, clean-host installation, Intel packaging, upgrade/rollback, or release acceptance.
