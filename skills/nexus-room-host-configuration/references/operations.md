# Room host operation guide

Use only operations returned by the live `rooms` or `emotion` definition.

| Operation | Input focus | Effect |
|---|---|---|
| `update_profile` | Optional name, description, avatar | UI immediate; runtime prompt next round |
| `set_collaboration_policy` | Optional Room Skill names, host auto-reply, private messaging | Security immediate; runtime next round |
| `add_member` | Existing owner-scoped `agent_id` | Membership immediate |
| `remove_member` | Current member `agent_id` | Revocation and active-task interruption immediate |
| `set_member_participation` | Current member `agent_id` plus explicit `paused` | CAS/authority fence immediate; pause interrupts, resume wakes pending work |
| `transfer_host` | Existing member `agent_id` | Authority immediate; routing next input |
| `create_conversation` | Optional title | UI immediate; runtime on next input |
| `update_conversation` | Current Room `conversation_id` and title | UI immediate; runtime next round |
| `delete_conversation` | Current Room `conversation_id` | Runtime close, artifacts/Goal cleanup, and deletion immediate |
| emotion `set_context` / `clear_context` | Current Agent/current conversation only | Next round |

The Room target is runtime-fixed: omit it for host operations. A host cannot call the owner-only `rooms.create` or `rooms.delete` operations from a Room, and cannot pause itself from its own Room round. After `transfer_host`, removing the host, or any member participation change, discard stale plans and inspect again.
