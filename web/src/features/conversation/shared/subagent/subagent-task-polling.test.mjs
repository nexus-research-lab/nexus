// @vitest-environment node

import assert from "node:assert/strict";

import { test } from "vitest";

test("subagent task changes refresh only the exact source and task", async () => {
  const {
    isSubagentTaskChangeFor,
  } = await import("@/features/conversation/shared/subagent/subagent-task-model.ts");
  const sessionSource = {
    kind: "session",
    session_key: "agent:dev:ws:dm:conversation-1",
  };
  const event = {
    agent_id: "agent-dev",
    data: { task_ids: ["task-market-research"] },
    delivery_mode: "ephemeral",
    event_type: "subagent_task_changed",
    protocol_version: 2,
    session_key: sessionSource.session_key,
    timestamp: Date.now(),
  };

  assert.equal(isSubagentTaskChangeFor(event, sessionSource), true);
  assert.equal(
    isSubagentTaskChangeFor(event, sessionSource, "task-market-research"),
    true,
  );
  assert.equal(isSubagentTaskChangeFor(event, sessionSource, "task-other"), false);
  assert.equal(
    isSubagentTaskChangeFor(event, { ...sessionSource, session_key: "other" }),
    false,
  );
  assert.equal(isSubagentTaskChangeFor(event, sessionSource, null, "agent-other"), false);

  const roomEvent = {
    ...event,
    conversation_id: "conversation-room",
    room_id: "room-1",
    session_key: "room-session",
  };
  assert.equal(isSubagentTaskChangeFor(roomEvent, {
    kind: "room",
    conversation_id: "conversation-room",
    room_id: "room-1",
  }, null, "agent-dev"), true);
  assert.equal(isSubagentTaskChangeFor(roomEvent, {
    kind: "room",
    conversation_id: "conversation-other",
    room_id: "room-1",
  }), false);

});
