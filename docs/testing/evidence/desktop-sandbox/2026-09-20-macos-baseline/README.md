# Fixed SDK and macOS desktop baseline (2026-09-20)

The fixed cross-repository baseline passed on macOS arm64 without a model
request. The run used Nexus `29ba7eef95fc688b41aa50fd16d1c6bc84f276a1`, SDK
`9d60e1661bc05fe516e7db0ac4e346c8c88a2b0f`, and Bridge
`02fbc0e5f6a699fad7106e202d119c272ef4e170` through the exact module
`v0.1.34-0.20260920021254-02fbc0e5f6a6` with checksum
`h1:4sQoNTQUHwidSCAP6DawenSZcrKIhwf23Gm542GI2XU=`.

The nxs binary was exported from the clean SDK commit and built by the gate.
Its SHA-256 is
`45525bf29672249dc2a99eb4cb88ccd7e8fa6b9d5fb7623005546c34d34d1ce8`.

## Command

```text
GOWORK=off GOPROXY=off node scripts/desktop/check-sandbox-baseline.mjs \
  --sdk-source /Users/berhand/program/Work/Nexus/worktrees/nexus-agent-sdk-go/desktop-sandbox \
  --sdk-ref 9d60e166
```

The command exited 0 and produced `report.json` with `passed: true` and
`releaseAccepted: false`. All 38 checks exited 0, including host policy,
host lifecycle, settings recovery, Provider environment, settings-writes,
macOS file/search/media/Skill/context/project/managed-policy paths, and the
macOS native integration group. The selected raw logs and their checksums are
kept beside the report; the full unfiltered temporary report directory is
recorded in the local command output.

## Scope and limits

This is a repeatable development baseline for the fixed local dependency set.
It proves no-model local host and macOS behavior for the listed tests. It does
not prove signed installation, clean-host upgrade/rollback, native Windows or
Linux execution, authenticated Claude command/network behavior, arbitrary
secret-file or inherited-handle isolation, complete external MCP/helper
confinement, or production release acceptance. `releaseAccepted` remains
`false`.
