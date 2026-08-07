# Owner operation guide

Load this reference only after `inspect_nexus_configuration` identifies a relevant domain. The returned definition remains authoritative if this guide differs.

## Domain routing

| Domain | Owner-main capability | Typical effect |
|---|---|---|
| `preferences` | Update chat delivery, runtime defaults, WebSearch, diagnostics, default model selections | Immediate or next round |
| `providers` | Create/update/delete private Providers; refresh, test, and configure models | Immediate tests; runtime next round |
| `agents` | Create/update/delete owner Agents and their runtime/tool/MCP options | Permission revocation immediate; most settings next round |
| `emotion` | Set the current Agent's base or DM-context emotion | Next round |
| `channels` | Configure encrypted Channel accounts, routing, and pairings | Runtime reload immediate; pairing next ingress |
| `connectors` | Direct credential connections and owner OAuth clients | Next session |
| `skills` | Search/import/update sources and install or disable Skills per Agent | Catalog immediate; runtime next round |
| `host` | Read a redacted startup/deployment snapshot and checks only | External change plus restart |
| `automation` | Use `nexus_automation`, not guessed `nexus_config` operations | Scheduler-defined |
| `sessions` | Rename or safely delete owner-scoped Agent sessions | UI or deletion immediate |
| `rooms` | Create/delete Rooms; manage profile, policy, members, host, and conversations | Security changes immediate; prompt next round/input |
| `workspaces` | Use `nexus_manager` workspace tools | Tool-defined |
| `goals` | Use `nexus_goal` | Goal runtime-defined |

## Secrets and approval

- Never place a real token, password, private header, OAuth secret, or credential in a configuration tool call.
- Use one opaque slot ID per secret leaf. The apply plan declares the slots; the human fills them only in the native permission surface.
- Destructive, credential-bearing, network, or authority-changing operations may require explicit native approval. A conversational “yes” does not replace it.
- `list_nexus_configuration_changes` is redacted and scope-filtered. Use it for audit/reconciliation, not as a secret store.

## Authorization flows

- Connector: start a durable flow only after the native human permits the start tool; retain only its opaque `flow_id`; query status or cancel by that ID.
- Channel: start the Channel authorization flow; QR, verification URL, and code entry stay in the native card; query status and cancel through the flow tools.
- After completion, inspect the relevant domain and verify the committed configuration version and runtime state.

## Failure handling

- `revision conflict` or plan digest mismatch: inspect and plan again.
- expired approval or round lease: start from inspect in the active round.
- partial update or uncertain write: inspect with verification, review audit history, and reconcile with a new request ID.
- runtime reload failure: preserve or restore the old runtime where the tool reports rollback; do not claim the database write alone means the feature is usable.
