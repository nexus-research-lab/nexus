# Bridge Claude probe cleanup and Nexus pin (2026-09-21)

This local evidence records the Nexus worktree after pinning the Bridge to
`37434c2d38b129b6bbde67ac81afee673f39816d`.

| item | value |
| --- | --- |
| SDK | `9956def130da33af47accf799a9c27c16a551104` |
| Bridge | `37434c2d38b129b6bbde67ac81afee673f39816d` |
| Nexus module | `v0.1.34-0.20260921030131-37434c2d38b1` |
| module checksum | `h1:XNPbPT5jcaMvs1sN5l4UmKZrL70Nre2BjmryNB4ffio=` |
| nxs binary | `/private/tmp/nxs-desktop-9956-bridge374` (SHA-256 in `nxs.sha256`) |

The Bridge target and transport race tests include the Claude restricted and
native-settings probe cleanup test. The probe now runs in a process session
(and the Windows implementation uses its Job boundary) and treats surviving
probe descendants as admission failure evidence. Nexus clientopts/runtime
checks, race, vet, and `make check-desktop-sandbox` all exited zero with the
local file module proxy.

This is a local development gate. The probe uses a fixture child process and
no model request. It does not prove an authenticated Claude command's native
file/network enforcement, Provider or secret-file isolation, arbitrary handle
cleanup, clean-host Windows/Linux behavior, signed packages, or release
acceptance. `releaseAccepted=false` remains required.
