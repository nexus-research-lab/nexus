# Agent self operation guide

The live inspect result is authoritative. This reference is a compact routing guide.

| Domain | Permitted operations | Boundary/effect |
|---|---|---|
| `agents` | `update_self_profile`, `update_self_runtime` | Current Agent only; profile updates UI now/runtime next round, runtime selection next round |
| `providers` | Inspect only | Credentials are never returned; choose only an enabled visible model |
| `skills` | `install_self`, `uninstall_self` | Current Agent only; use exact `target_scope` and `source_identity`; next round |
| `emotion` | `set_base`, `set_context`, `clear_context` | Current Agent/current DM only; fatigue remains runtime-owned; next round |
| `sessions` | `update_title` | Current trusted DM only; UI immediate |

All other configuration domains are denied in an ordinary Agent DM.

Always inspect first because an owner may have removed a Provider, Skill, or permission since the previous round. For `update_self_runtime`, never send Provider credentials; submit only an enabled provider key, model ID, and optional turn/thinking limits. For Skill changes, never substitute a same-name source: preserve the exact source identity returned by inspect.
