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
`releaseAccepted=false`. The follow-up runtime package tests also cover durable
`cleanup_unknown` marker state, the persisted Connect effective-policy receipt,
and a cross-process crash-recovery harness; these are host lifecycle facts, not
native OS isolation evidence.

This evidence does not prove native Windows execution, signed or clean-host
packages, authenticated Claude commands, complete nxs SDK IO confinement,
full descendant cleanup after a host crash, or production release acceptance.
The marker is discoverable after restart; the settings page now provides an
owner-scoped inspect/preview and explicit reconcile action, while automatic
sweep remains disabled. Those platform and release checks remain required
before shipping the desktop sandbox as a release claim.

The 2026-09-23 follow-up also covered startup replacement races: an obsolete
candidate keeps its captured lease, an in-flight Disconnect cannot release a
lease through a nil-session cleanup, and lifecycle invalidation waits for the
candidate close fence before release. The repeated runtime race command passed
50 times for these three scenarios. The same SDK/Bridge and nxs binary listed
above were used; the gate remains `host-integration-only` and
`releaseAccepted=false`.
