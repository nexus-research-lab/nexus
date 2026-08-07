---
name: nexus-agent-self-configuration
description: Inspect and change the current ordinary Nexus Agent's permitted self settings from its private DM. Use when the user asks this Agent to update its own profile, model/runtime limits, Skills, emotion, or current session title, or to explain why an owner, host, Channel, Connector, Provider, or system setting cannot be changed here.
---

# Nexus Agent Self Configuration

Operate only on the current Agent and current trusted private DM. Nexus resolves identity and scope from the runtime; tool arguments cannot expand them.

## Workflow

1. Call `inspect_nexus_configuration` for the relevant domain and follow the returned `access` and operation schema.
2. For a permitted mutation, call `plan_nexus_configuration_change` with the current Agent target omitted unless the returned contract requires it.
3. Explain the effect and when it reloads. Call `apply_nexus_configuration_change` with a fresh `request_id`, exact revision, and plan digest. Native approval is required when the plan says so.
4. Verify the returned checks or inspect again. Re-plan on conflicts; never force a stale write.
5. For sensitive fields, use only opaque secret placeholders and let Nexus collect values in the native card. Never request secrets in chat.

Read [references/operations.md](references/operations.md) only when deciding an exact self operation or denial boundary.

## Boundaries

- Allowed self areas are the current Agent's profile, safe runtime selection/limits, owner-visible Skill installation state, base/DM-context emotion, and current session title.
- Provider catalog inspection is read-only so a valid model can be selected. This Agent cannot create, edit, test, or delete Providers.
- This Agent cannot change owner preferences, other Agents, host settings, Channels, Connectors, imported Skill sources, automations, Goals, workspaces through configuration, Rooms from a DM, or another session.
- Do not use shell, database, files, environment variables, raw `nexusctl`, or MCP configuration to bypass a denial.
- If the user needs an owner-level change, say it must be requested from the Nexus main Agent's private DM. If it concerns a Room, continue in that Room under its current host/member authority.
- A revocation or role change takes effect immediately. Re-inspect rather than relying on an earlier plan.

## Response

Report the exact self resource changed, next-round or immediate effect, and verification. On denial, state the correct context and authority in one sentence.
