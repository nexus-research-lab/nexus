# Fixed SDK + macOS desktop baseline (2026-09-20, latest Bridge)

The fixed archive-built SDK baseline ran on macOS arm64 with no model request.
The run used Nexus commit `0e40cfb6a` plus the working-tree Bridge pin
`v0.1.34-0.20260920071621-8e90ff5e35e3` (Bridge commit `8e90ff5e35e3`)
and SDK commit `9956def130da33af47accf799a9c27c16a551104`.

The command exited 0. `report.json` records all 38 checks and the exact
commands. Settings writers produced 47 passing events, including the
cross-physical-root mixed-state fail-closed case. The generated nxs SHA-256 is
`9896f72b69797b180d24274f861ed5fca77f153f9c54e6a0796d700251d894ac`; the exported SDK archive SHA-256 is `9c203f071ad1210ba3351b57bf4e1d8ee4b801d9c652ed8b92ba861e9bdf307e`.

The runner's nxs binary and SDK archive are omitted from Git; their hashes and
the temporary source directory are recorded in `omitted-artifacts.txt`.
The selected raw logs are kept beside the report.

## Scope and limits

This is local development evidence with `passed: true` and
`releaseAccepted: false`. It does not prove signed packages, clean-host
upgrade/rollback, native Windows or Linux execution, authenticated Claude
command/network behavior, arbitrary secret-file or inherited-handle isolation,
complete external MCP/helper confinement, or production release acceptance.
