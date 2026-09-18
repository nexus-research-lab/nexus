import { describe, expect, it } from "vitest";

import type {
  AssistantMessage,
  Message,
} from "@/types/conversation/message/entity";

import {
  latestAssistantResultFailure,
  resolveAssistantFailureCode,
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
  it("classifies context/token exhaustion as a usage limit", () => {
    expect(resolveAssistantFailureCode(assistantFailure("round-1")))
      .toBe("usage_limited");
  });

  it("does not restore an old assistant failure after a newer user message", () => {
    const messages: Message[] = [
      assistantFailure("round-1"),
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
    ];

    expect(latestAssistantResultFailure(messages)).toBeNull();
  });

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

  it("keeps the failure when only an internal hidden user record follows it", () => {
    const messages: Message[] = [
      assistantFailure("round-1"),
      {
        agent_id: "nexus",
        content: "internal continuation",
        hidden_from_user: true,
        is_synthetic: true,
        message_id: "internal-1",
        role: "user",
        round_id: "round-1",
        session_key: "session-1",
        timestamp: 3,
      },
    ];

    expect(latestAssistantResultFailure(messages)?.round_id).toBe("round-1");
  });
});
