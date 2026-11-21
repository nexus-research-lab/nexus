import { describe, expect, it } from "vitest";

import type { Message } from "@/types/conversation/message/entity";

import { boundMessageWindow } from "./message-window-model";

function round(index: number, overrides: Partial<Message> = {}): Message {
  return {
    agent_id: "amy",
    content: `user-${index}`,
    message_id: `user-${index}`,
    role: "user",
    round_id: `round-${index}`,
    session_key: "agent:amy:ws:dm:test",
    timestamp: index,
    ...overrides,
  } as Message;
}

function rounds(from: number, to: number): Message[] {
  return Array.from({ length: to - from + 1 }, (_, offset) => round(from + offset));
}

function roundIds(messages: Message[]): string[] {
  return messages.map((message) => message.round_id);
}

const budget = { maxBytes: Number.MAX_SAFE_INTEGER, maxRounds: 4 };

describe("bounded continuous message window", () => {
  it("evicts whole rounds from one edge only and records where the window detaches from latest", () => {
    const head = boundMessageWindow(
      { gapAfterRoundId: null, messages: rounds(1, 6) },
      { ...budget, evict: "head" },
    );
    expect(roundIds(head.messages)).toEqual(["round-3", "round-4", "round-5", "round-6"]);
    expect(head.gapAfterRoundId).toBeNull();

    const tail = boundMessageWindow(
      { gapAfterRoundId: null, messages: rounds(1, 6) },
      { ...budget, evict: "tail" },
    );
    expect(roundIds(tail.messages)).toEqual(["round-1", "round-2", "round-3", "round-4"]);
    expect(tail.gapAfterRoundId).toBe("round-4");
  });

  it("keeps streaming rounds resident behind the gap when the tail is evicted", () => {
    const streaming = round(9, {
      content: [],
      is_complete: false,
      message_id: "assistant-9",
      role: "assistant",
      stream_status: "streaming",
    } as Partial<Message>);
    const bounded = boundMessageWindow(
      { gapAfterRoundId: null, messages: [...rounds(1, 6), streaming] },
      { ...budget, evict: "tail" },
    );
    expect(roundIds(bounded.messages)).toEqual(["round-1", "round-2", "round-3", "round-9"]);
    expect(bounded.gapAfterRoundId).toBe("round-3");
  });

  it("never leaves more than one gap", () => {
    // 窗口 1..3 之后有断点，尾部 7..9 连到最新。
    const detached = {
      gapAfterRoundId: "round-3",
      messages: [...rounds(1, 3), ...rounds(7, 9)],
    };

    const tail = boundMessageWindow(detached, { ...budget, evict: "tail" });
    expect(roundIds(tail.messages)).toEqual(["round-1", "round-2", "round-3", "round-9"]);
    expect(tail.gapAfterRoundId).toBe("round-3");

    const head = boundMessageWindow(detached, { ...budget, evict: "head" });
    expect(roundIds(head.messages)).toEqual(["round-3", "round-7", "round-8", "round-9"]);
    expect(head.gapAfterRoundId).toBe("round-3");

    const headOnly = boundMessageWindow(detached, { ...budget, evict: "head", maxRounds: 2 });
    expect(roundIds(headOnly.messages)).toEqual(["round-8", "round-9"]);
    expect(headOnly.gapAfterRoundId).toBeNull();
  });

  it("drops a gap whose round is no longer resident and keeps an oversized round atomic", () => {
    const window = { gapAfterRoundId: "round-0", messages: rounds(1, 2) };
    expect(boundMessageWindow(window, budget).gapAfterRoundId).toBeNull();

    const oversized = boundMessageWindow(
      {
        gapAfterRoundId: null,
        messages: [
          round(1, { content: "a".repeat(2_000) }),
          round(2, { content: "b".repeat(2_000) }),
        ],
      },
      { evict: "tail", maxBytes: 512, maxRounds: 10 },
    );
    expect(roundIds(oversized.messages)).toEqual(["round-1"]);
    expect(oversized.gapAfterRoundId).toBe("round-1");
  });
});
