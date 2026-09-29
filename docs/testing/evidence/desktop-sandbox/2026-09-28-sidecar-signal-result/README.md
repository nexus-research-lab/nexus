# macOS sidecar signal result handling

Nexus baseline `8f3a9cade` plus this change. Bridge `c994b197e010`, SDK `db867f87`. Host macOS 27 arm64; all Go commands used `GOWORK=off`.

The native libproc signal interface returns positive errno codes. Swift now uses that returned code directly, and only consults errno for negative returns. This prevents stale ESRCH from hiding a real permission failure and accepts an explicitly absent exact target even when errno is unset. Errors continue to retain the recovery record.

## Verification

- Ten reaper test bodies passed through the archived standalone Swift assertion harness, including positive/negative error results, stale errno, native stale-token denial, exact termination, orphan cleanup and legacy/unknown records. XCTest remains unavailable; this is not an XCTest result.
- Production Swift release build passed.
- Fresh sidecar and pinned Bridge helper bundle built; hashes are in `binaries.json`. A bundled arm64 App with the fixed nxs and ripgrep sidecars passed `check-macos-runtime.sh`; its compatibility report is `bundled-runtime-check.json`.
- The pinned SDK native macOS baseline passed 10 checks and 100 required test names, including workspace aliases, ask/deny, symlink and protected-write cases; `native-macos-baseline-report.json` records the `host-and-macos-native-baseline` scope. The host integration and explicit released-vs-candidate runtime upgrade gates also passed all required phases; `runtime-upgrade-report.json` records both hashes and keeps `releaseAccepted=false`.
- A real third-party nxs DM spawned a detached child through the freshly built default sidecar. After SIGKILL of its fixture host, the child survived; restarting the same fixture state retired it and reconciled the original process, scratch and policy records without replay. Only scoped before/after receipts are archived; no provider configuration, database or complete fixture state is copied.

Reproduce the Swift checks:

```sh
swiftc desktop/macos/Sources/NexusDesktop/Sidecar/SidecarProcessIdentity.swift \
  desktop/macos/Sources/NexusDesktop/Sidecar/SidecarOrphanReaper.swift \
  docs/testing/evidence/desktop-sandbox/2026-09-28-sidecar-signal-result/tests.swift \
  docs/testing/evidence/desktop-sandbox/2026-09-28-sidecar-signal-result/main.swift \
  -o /tmp/nexus-sidecar-signal-result-check
/tmp/nexus-sidecar-signal-result-check
```

## Remaining boundary

This fixes return-code interpretation; it does not make the missing macOS 14.0 exact-signal API available. Minimum version remains unchanged. Full Access deliberately has no filesystem isolation guarantee. Graphical App acceptance, remaining SDK IO/secret/IPC/egress boundaries, native MCP race initialization and signed/clean-host/Intel/upgrade release acceptance remain separate. `releaseAccepted=false`.
