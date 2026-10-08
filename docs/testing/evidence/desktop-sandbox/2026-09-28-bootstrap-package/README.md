# macOS bootstrap packaging — 2026-09-28

Base Nexus commit: `29d91aec0`, plus the packaging/loader changes committed with this evidence.
Bridge: `v0.1.34-0.20260928032121-5e6db3dc4bef`. Host: macOS arm64.

- Actual `build-macos-app.sh` assembly passed using `NEXUS_DESKTOP_APP_NAME=NexusBootstrapQA`, isolated `/tmp/nexus-bootstrap-app-20260928`, arm64, default ad-hoc signing.
- Helper built from the pinned module with `GOWORK=off`, native cgo. Manifest generated after helper signing and before App signing; final manifest verification passed.
- `codesign --verify --deep --strict --verbose=2` passed (see signature.txt).
- Node native manifest tests passed without skips. Go loader native race tests and pinned-dependency rejection tests passed. Targeted Go vet, architecture gate and shell syntax passed.
- The App was not started and nxs was not bundled. This proves assembly/signing order, not runtime integration, nxs pairing, Developer ID, notarization, clean-host, Intel or macOS 14.0 compatibility.
- Default supervisor wiring, startup recovery and scratch binding remain open.

All Go commands used `GOWORK=off`. No Windows validation was run.

Build log trailing whitespace is normalized; message content is unchanged.
