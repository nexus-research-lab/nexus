import { describe, expect, it } from "vitest";

import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";

import {
  latestAssistantResultFailure,
} from "./assistant-message-model";

const assistantFailure = (roundId: string): AssistantMessage => ({
  agent_id: "nexus",
  content: [],
  is_complete: true,
  message_id: `assistant-${roundId}`,
  result_summary: {
    duration_api_ms: 0,
    duration_ms: 0,
    errors: ["context_length_exceeded"],
    is_error: true,
    num_turns: 0,
    subtype: "error",
  },
  role: "assistant",
  round_id: roundId,
  session_key: "session-1",
  timestamp: 2,
});

describe("assistant result failure projection", () => {

  it("ignores a late failure snapshot whose timestamp predates a newer send", () => {
    const messages: Message[] = [
      {
        agent_id: "nexus",
        client_message_id: "client-2",
        content: "继续",
        message_id: "user-2",
        role: "user",
        round_id: "client-2",
        session_key: "session-1",
        timestamp: 3,
      },
      assistantFailure("round-1"),
    ];

    expect(latestAssistantResultFailure(messages)).toBeNull();
  });
});
