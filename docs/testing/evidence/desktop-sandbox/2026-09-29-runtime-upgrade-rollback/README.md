# macOS runtime upgrade and rollback compatibility

- Date: 2026-09-29
- Nexus branch: `codex/desktop-sandbox-approvals`
- Script: `scripts/desktop/check-runtime-upgrade.mjs`
- Scope: `runtime-data-compatibility`
- Result: passed

The explicit previous/candidate runtime check passed all required phases:

```text
TestDesktopSandboxRuntimeUpgradeAndRollback/before_upgrade PASS
TestDesktopSandboxRuntimeUpgradeAndRollback/after_upgrade PASS
TestDesktopSandboxRuntimeUpgradeAndRollback/after_rollback PASS
```

The previous bundled nxs SHA-256 was
`63aac0dcc63568820b234d06632e3d4af29855842ddc878e37c47c6bb0bf64b1` and the
candidate nxs SHA-256 was
`1f863787e53eccee968bd3fc277c220bf7c625a99c2c23b4e1d88e175742ca1e`.

This proves local runtime data compatibility and rollback behavior with an
explicit pinned Bridge. It does not prove DMG installation, Developer ID,
notarization, Gatekeeper clean-host behavior, Intel packaging, or production
release acceptance; the script reports `releaseAccepted=false`.
