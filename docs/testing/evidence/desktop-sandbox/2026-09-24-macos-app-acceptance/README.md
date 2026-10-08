# macOS App packaging and native smoke evidence

This record covers the macOS arm64 developer-machine acceptance run for the
working tree at commit `e9a15522db53cafe8f653ce3ef43871acd34dfed`. The tree was
dirty; the complete dirty scope is the worktree state at the time of the run.
It is evidence for the current App/sidecar packaging path, not a release claim.

## Environment

- macOS 27.0 (26A428), arm64
- Swift 6.4, Go 1.27.1, Node 22.23.2, pnpm 12.3.4
- nxs input: `/tmp/nxs-desktop-9956`, SHA-256
  `0f91b17fc0ed6976e01a76363f466640a1cddfa63bc32338cb7647153e014270`
- rg input: `/tmp/rg`, SHA-256
  `468681470b3ed6291dc85691e83c5d2f8fc6c6ec25b164ffa66248713a7e28a3`

The packaged binaries are ad-hoc signed after copying. Their post-signature
hashes are recorded in `runtime-sha256.txt`; they therefore differ from the
unsigned input nxs/rg hashes above.

## Results

1. Bundled arm64 App build and smoke passed (exit 0):

   ```text
   GOWORK=off \
   NEXUS_DESKTOP_APP_BUILD_DIR=/tmp/nexus-macos-bundled-app \
   NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME=1 \
   NEXUS_DESKTOP_NXS_RUNTIME_PATH=/tmp/nxs-desktop-9956 \
   NEXUS_DESKTOP_SMOKE_ALLOW_FALLBACK=1 \
   NEXUS_DESKTOP_SMOKE_MAIN_TIMEOUT_SECONDS=45 \
   NEXUS_DESKTOP_SMOKE_LAUNCHER_TIMEOUT_SECONDS=30 \
   make app-check
   ```

   The run verified the Web build, Swift shell, Go sidecar, nexusctl/nexuscfg,
   bundled nxs/rg, credentials storage, main/Launcher routing, URL/notification
   fallback, clean exit, and sidecar termination.

2. The ad-hoc DMG was built with the bundled nxs runtime (exit 0):

   `Nexus-macos-arm64-0.2.0-2457.dmg` is recorded in `dmg.metadata.json` and
   its SHA-256 is verified by both `dmg.sha256` and `dmg-computed-sha256.txt`.
   The metadata reports arm64, `bundled: true`, `signing.kind: ad-hoc`,
   `developer_id: false`, and `notarized: false`.

3. The DMG was mounted read-only and the contained App passed
   `codesign --verify --deep --strict`; its arm64 nxs and rg were present.
   `spctl --assess --type execute` returned 0 on this development host with
   `override=security disabled`. This is local Gatekeeper state, not clean-host
   acceptance.

4. Smoke was run directly from the mounted DMG with
   `NEXUS_DESKTOP_SMOKE_EXPECT_NXS_RUNTIME=1` and passed (exit 0); the image was
   detached afterwards.

5. The native UI harness passed all 12 app-shell cases across light/dark/rain,
   English/Chinese, and 1280/360 widths. `ui-report.json` records zero rejected
   business requests, 12 socket connections, trusted native input, zoom/
   unzoom, resize, and resume checks.

6. A local install rehearsal copied the bundled App assembled for the DMG into a temporary
   Applications directory, launched it with a new state root, replaced it once,
   and rolled the previous copy back. All three smoke runs exited 0. This
   demonstrates that the bundle does not depend on the build directory, while
   remaining a same-host rehearsal rather than clean-host or versioned upgrade
   acceptance.

## Limits

- This is an ad-hoc, dirty-tree, single-host arm64 result. It does not prove
  Developer ID signing, notarization, clean-host installation, quarantine,
  Intel compatibility, upgrade/rollback, or production release acceptance.
- No real external Provider request was sent. The nxs hash proves the fixed
  input binary used for this run, not the correctness of all SDK IO, network,
  secret-file, handle, helper-process, or descendant-process isolation.
- Official Claude account/OAuth authentication is intentionally outside this
  phase; third-party provider compatibility remains a separate host-integration
  check.
