---
name: nexus-room-host-configuration
description: Configure and verify the current Nexus Room when the executing Agent is its database-backed host. Use in a Room conversation when users ask to change the Room profile, collaboration policy, Skills, host behavior, private messaging, membership or participation, host assignment, conversations, or the current Agent's Room-context emotion.
---

# Nexus Room Host Configuration

Manage only the current Room. Nexus revalidates the host membership and active Room execution slot on every call; this Skill does not grant host authority.

## Workflow

1. Call `inspect_nexus_configuration` for `rooms` or `emotion` and read the current Room snapshot, access, version, and exact operation contract.
2. Call `plan_nexus_configuration_change` with the Room target omitted so the runtime-fixed current Room remains the scope. Include only IDs obtained from current Room/Agent inspection.
3. Explain membership, routing, visibility, interruption, and reload effects. Then apply with a fresh request ID and the exact revision/digest through the native approval surface.
4. Verify checks and re-inspect the Room. On host transfer or membership removal, treat the new authority as immediate and stop using stale host capability.

Read [references/operations.md](references/operations.md) only for exact host operations and effect timing.

## Boundaries

- The host may update the current Room profile and collaboration policy; add/remove current-owner Agents; pause or resume another member's participation; transfer host to an existing member; and create, rename, or delete conversations in this Room.
- The host cannot pause itself from its own Room round. Transfer host first, or ask the owner-main Agent to apply that change, so conversational control cannot lock itself out.
- The host cannot delete the whole Room from inside the Room, manage another Room, or change owner-global preferences, Providers, Agent runtime settings, Channels, Connectors, imported Skill sources, host deployment, sessions, workspaces, automations, or Goals through this configuration capability.
- `emotion` changes only the current executing Agent's context in the current conversation. Host status does not permit rewriting another Agent's identity or emotion.
- Removal and authority changes are security-immediate. Removed members' active Room work is interrupted; private/public delivery must be reauthorized before output.
- Do not bypass a denied operation with files, database access, shell, environment changes, or raw control-plane CLI.

## Response

Report the current Room/conversation affected, the authority or membership transition, reload timing, and verification. If this Agent is no longer host, say so and continue under member rules.
