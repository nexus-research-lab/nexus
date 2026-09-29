# Bundled macOS runtime compatibility check

- Date: 2026-09-29
- Nexus branch: `codex/desktop-sandbox-approvals`
- App input: `desktop/macos/.build/app/Nexus.app`
- Command: `scripts/desktop/check-macos-runtime.sh <absolute-app> <report>`
- Result: passed

The bundled sidecar accepted the packaged nxs runtime and reported:

```json
{"version":1,"platform":"darwin","architecture":"arm64","runtime_sha256":"63aac0dcc63568820b234d06632e3d4af29855842ddc878e37c47c6bb0bf64b1","bridge":"v0.1.34-0.20260928040950-c994b197e010","profiles":["workspace-write","read-only","full-access"]}
```

This proves the package's internal runtime compatibility handshake for the
existing arm64 development bundle. It is not Developer ID signing,
notarization, Gatekeeper clean-host installation, Intel packaging, upgrade or
rollback acceptance, or graphical App acceptance.
