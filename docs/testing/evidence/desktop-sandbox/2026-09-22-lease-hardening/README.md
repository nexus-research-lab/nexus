# Desktop sandbox lease hardening evidence (2026-09-22)

This directory records the local verification after the exact lease transfer,
duplicate-release serialization, unclean-discard cleanup, and Windows marker
hard-link checks were hardened.

| item | value |
| --- | --- |
| SDK | `9956def130da33af47accf799a9c27c16a551104` |
| Bridge | `37434c2d38b129b6bbde67ac81afee673f39816d` |
| nxs binary | `/private/tmp/nxs-desktop-9956-bridge374` (SHA-256 in `nxs.sha256`) |
| host | macOS arm64, Go 1.27.1 |

`source.json` records the exact dependency and binary identities. Compressed
stdout/stderr captures accompany the report for the target race, service race,
vet, architecture, and desktop host groups.

The target package race tests cover runtime, confined filesystem, client
options, DM, Room realtime, and AutoDream. The architecture check, targeted
vet, fixed nxs resource-backed handshake, and desktop host gate all exited
zero. Windows amd64 and macOS arm64 test binaries for runtime and confinedfs
also cross-compiled successfully; `cross-compile.json` records the exact
commands and artifact hashes. The gate report is explicitly `host-integration-only` with
`releaseAccepted=false`.

This evidence does not prove native Windows execution, signed or clean-host
packages, authenticated Claude commands, complete nxs SDK IO confinement,
effective-policy receipts, crash recovery across host restarts, or production
release acceptance. Those remain required before shipping the desktop sandbox
as a release claim.
