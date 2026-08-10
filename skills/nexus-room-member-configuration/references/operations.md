# Room member operation guide

The live inspect response is authoritative.

| Domain | Member capability | Effect |
|---|---|---|
| `rooms` | Read the current Room's safe projection only | No mutation |
| `emotion` | `set_context` or `clear_context` for this Agent and current conversation | Next round |

Requests to update Room profile, collaboration policy, Skills, host auto-reply, private messaging, membership, host, or conversations require the current Room host. Whole-Room creation/deletion and global Nexus configuration require the Nexus main Agent in its private DM. Never forward a stale plan or request ID across those context changes; the authorized Agent must inspect and plan in its own active round.
