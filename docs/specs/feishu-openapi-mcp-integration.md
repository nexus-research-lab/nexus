# Feishu OpenAPI MCP Integration

## Why This Exists

Nexus scheduled tasks must deliver through the Nexus Feishu channel at runtime so run ledgers, delivery status, retries, dead letters, audit events, and daily reports remain authoritative.

The official Feishu/Lark OpenAPI MCP is still useful, but as a documentation and API-understanding aid rather than the primary scheduled-task delivery path.

Use it for:

- checking which Feishu OpenAPI endpoint or permission is needed;
- explaining errors such as tenant token failure, missing permission, wrong `receive_id_type`, or invalid `chat_id`;
- validating future Feishu channel expansion before coding it into Nexus.

Do not use it for:

- bypassing `nexus_automation` delivery from a scheduled task;
- sending a scheduled-task result directly from an Agent without recording the delivery attempt in Nexus;
- storing App Secret or OAuth tokens in repo files.

## Official MCPs

Repository: <https://github.com/larksuite/lark-openapi-mcp>

Hermes/OpenClaw reference plugin: <https://github.com/larksuite/openclaw-larksuite>

The Hermes-facing Lark plugin listed by OpenClaw Directory is an adaptation of the official OpenClaw Lark/Feishu plugin. Its useful product signal for Nexus is broad Feishu tool coverage plus OAuth/multi-user support, but scheduled-task result delivery should still remain inside Nexus so the run ledger stays authoritative.

Documentation recall MCP:

```json
{
  "mcpServers": {
    "lark-docs": {
      "command": "npx",
      "args": ["-y", "@larksuiteoapi/lark-mcp", "recall-developer-documents"]
    }
  }
}
```

OpenAPI MCP for manual development experiments:

```json
{
  "mcpServers": {
    "lark-mcp": {
      "command": "npx",
      "args": [
        "-y",
        "@larksuiteoapi/lark-mcp",
        "mcp",
        "-a",
        "<feishu_app_id>",
        "-s",
        "<feishu_app_secret>",
        "-t",
        "im.v1.message.create,im.v1.message.list,im.v1.chat.get"
      ]
    }
  }
}
```

The documentation recall MCP needs Node.js and can run in stdio mode by default. The OpenAPI MCP needs a real Feishu/Lark app, App ID, App Secret, and the required permissions.

## Runtime Boundary

For scheduled tasks, the production path remains:

1. Agent creates or updates a task through `nexus_automation`.
2. Automation runtime executes the run and stores execution output.
3. Nexus Feishu channel sends text through `open-apis/im/v1/messages`.
4. Delivery metadata is recorded on `automation_cron_runs`.
5. Retry scheduler or `retry_scheduled_task_delivery` handles delivery-only failure.

If an Agent uses `lark-mcp` to send a message directly, Nexus cannot prove whether a scheduled-task result was delivered. That breaks the long-running stability model.

## Troubleshooting Guide

When Nexus reports a Feishu delivery error:

- `feishu channel is not configured`: App ID / App Secret are missing or channel config is disabled.
- `feishu tenant_access_token failed`: check app credentials and tenant/app status.
- `feishu send message failed`: inspect Feishu error code, permission, `receive_id_type`, and target chat/user id.
- repeated delivery failure with `delivery_dead_letter_at`: use `get_scheduled_task_status`, then `retry_scheduled_task_delivery` after fixing the channel config or target.

When API semantics are unclear, query `lark-docs` first with the exact endpoint or error text, then update Nexus code or configuration. Keep the final scheduled-task action inside Nexus tools.

## Future Product Direction

Nexus should eventually support declaring optional MCP dependencies for managed skills. For Feishu scheduled-task work, the desired dependency is:

- `lark-docs`: official Feishu/Lark developer documentation retrieval MCP;
- optional `lark-mcp`: bounded OpenAPI MCP for development and manual verification.

Until that exists, this document and `scheduled-task-manager` describe the manual integration boundary.
