import { describe, expect, it } from "vitest";

import type { Message } from "@/types/conversation/message/entity";

import {
  advanceWindowGap,
  boundMessageWindow,
  pinnedWindowRounds,
} from "./message-window-model";

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

describe("navigator jump into a detached window", () => {
  const detached = { gapAfterRoundId: "round-4", messages: [...rounds(1, 4), round(9)] };
  const pageRoundIds = (from: number, to: number) => roundIds(rounds(from, to));

  it("moves the gap to the end of a page that extends the window past it", () => {
    // 断点留在 round-4 会让向后加载把 5、6 误判为已接上最新，7、8 再也加载不到。
    expect(advanceWindowGap(detached, pageRoundIds(3, 6))).toBe("round-6");
  });

  it("closes the gap once the page reaches the rounds resident behind it", () => {
    expect(advanceWindowGap(detached, pageRoundIds(3, 9))).toBeNull();
  });

  it("leaves the gap alone when the page stays on the resident side of it", () => {
    expect(advanceWindowGap(detached, pageRoundIds(1, 4))).toBe("round-4");
    expect(advanceWindowGap(detached, pageRoundIds(1, 2))).toBe("round-4");
    expect(advanceWindowGap({ ...detached, gapAfterRoundId: null }, pageRoundIds(3, 6))).toBeNull();
  });

  it("carries a streaming round over whole, with its prompt and finished replies", () => {
    const prompt = round(9);
    const finished = round(9, { message_id: "assistant-9a", role: "assistant" });
    const streaming = round(9, {
      is_complete: false,
      message_id: "assistant-9b",
      role: "assistant",
      stream_status: "streaming",
    } as Partial<Message>);
    expect(pinnedWindowRounds([...rounds(1, 4), prompt, finished, streaming]))
      .toEqual([prompt, finished, streaming]);
  });
});
