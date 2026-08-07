---
name: nexus-owner-configuration
description: Configure and verify owner-scoped Nexus settings from the Nexus main Agent's private DM. Use when the user asks to inspect, change, connect, enable, disable, test, or repair Providers, Agents, preferences, Channels, Connectors, Skills, sessions, Rooms, automations, models, tools, or MCP settings through conversation.
---

# Nexus Owner Configuration

Use the Nexus configuration and authorization tools as the control plane. The service, not this Skill, decides the current human role, resource scope, confirmation requirement, and whether a stale plan may apply.

## Workflow

1. Call `inspect_nexus_configuration` for only the relevant domains. Use `verify: true` when the user asks to check or repair behavior.
2. Read `access`, `definition.operations`, `current`, `revision`, and checks. Never guess an operation or input shape.
3. For a mutation, call `plan_nexus_configuration_change` with the exact target and non-secret input. Put every secret leaf in an opaque `{"$secret":"slot_id"}` placeholder.
4. Summarize the planned effect, scope, reload behavior, and destructive impact. Then call `apply_nexus_configuration_change` with a fresh `request_id` and the plan's exact `current_revision` and `plan_digest`. Nexus collects secrets and human approval in its native card; never ask the user to paste a token or password into chat.
5. Treat the apply result as provisional until its checks succeed. If verification is incomplete, inspect again and report `reconcile_required` instead of claiming success.
6. On a retry of the same network attempt, reuse `request_id`. For a changed intent or a new plan, use a new ID. On revision conflict, inspect and plan again; never overwrite.

Read [references/operations.md](references/operations.md) only when exact domain boundaries, authorization flows, or reload behavior are needed.

## Boundaries

- This Skill is active only for the Nexus main Agent in its own private DM. A Room always uses Room authority, even if the same Agent identity appears there.
- Owner configuration covers private owner resources. Public Provider or host administration still requires the authenticated owner/admin role enforced by Nexus.
- `host` is inspection-only. Change deployment environment, process startup policy, or desktop state root through the deployment/native desktop control plane and restart as instructed by checks. Never create a shadow JSON config.
- Use `nexus_config` for product configuration. Do not edit the database, runtime state, workspace files, environment variables, or invoke `nexusctl` as a substitute.
- Do not expose secrets from tool input, approval UI, audit history, URLs, headers, logs, or error messages.
- Connector OAuth/device authorization uses `start_connector_authorization`, status, and cancel tools. Channel QR/device authorization uses the corresponding Nexus Channel authorization tools. Never parse or repeat authorization codes, state, QR payloads, device codes, or tokens.
- Immediate revocation wins over conversational continuity. If an Agent, Room member, Channel, Connector, session, or permission is revoked, stop using the old capability and re-inspect.

## Response

State what changed, its scope, when it becomes effective (`immediate`, `next input`, `next round`, `next session`, or `restart`), and the verification result. If denied, name the required authority or correct context without proposing a bypass.
