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
| `skills` | Search sources; create/update/delete private registries; import/update Skills; install or disable Skills per Agent | Source/catalog results immediate; Agents reload content and selection next round |
| `host` | Read a redacted startup/deployment snapshot and checks only | External change plus restart |
| `automation` | Use `nexus_automation`, not guessed `nexus_config` operations | Scheduler-defined |
| `sessions` | Rename or safely delete owner-scoped Agent sessions | UI or deletion immediate |
| `rooms` | Create/delete Rooms; manage profile, policy, members, participation, host, and conversations | Security changes immediate; prompt next round/input |
| `workspaces` | Use the `nexus-manager` Skill and owner-scoped `nexusctl`; use native file tools only for the current Agent workspace | Immediate |
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

## Private Skill sources

Load this section only when the owner asks to manage a private Skill registry. These operations are owner-main only and must never be inferred in Agent-self or Room roles.

- Inspect `skills` first and copy the exact safe `source_id`, `managed_by`, `auth_type`, `credential_configured`, and `deletable` facts. Never request or repeat an existing credential value.
- `create_private_source`: omit the target; provide `name`, validated registry `url`, and `auth_type`. For `bearer`, put an opaque secret slot in `token`; the human enters the real token in the native approval card.
- `update_source`: use the exact inspected `source_id`. A system source allows only `enabled`; an owner-private source may change `name`, `enabled`, `auth_type`, or rotate a bearer `token` through a secret slot.
- `import_private`: use the exact private `source_id` plus the opaque `skill_id` returned by source search. Do not submit a browser-provided download URL.
- `delete_private_source`: use only a source marked `deletable`. Deleting the source preserves Skills already imported into the owner catalog.
- Every functional source write and import advances the same owner catalog version used by settings/API writes. On a revision conflict, inspect again; do not replay the stale plan. Health-check timestamps and errors are non-functional metadata and do not invalidate a plan.

## Failure handling

- `revision conflict` or plan digest mismatch: inspect and plan again.
- expired approval or round lease: start from inspect in the active round.
- partial update or uncertain write: inspect with verification, review audit history, and reconcile with a new request ID.
- runtime reload failure: preserve or restore the old runtime where the tool reports rollback; do not claim the database write alone means the feature is usable.
