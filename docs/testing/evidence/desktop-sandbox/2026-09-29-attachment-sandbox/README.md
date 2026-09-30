# 2026-09-29 attachment sandbox baseline

The SDK commit `b02ae892` routes user-message attachment metadata checks through the
active file executor before upload. Nexus `2c34b0546` and Bridge `c251a8d` were used for
the host-and-macos-native baseline on macOS 27.0 arm64. All 61 checks and 888 required
check names passed.

This is development evidence only. It does not prove graphical App end-to-end,
macOS 14.0 signal compatibility, signing/notarization, clean-host or Intel acceptance;
`releaseAccepted` remains `false`.

```text
node scripts/desktop/check-sandbox-baseline.mjs --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox --sdk-ref b02ae892
```
