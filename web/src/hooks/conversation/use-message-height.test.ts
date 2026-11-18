import { prepare } from "@chenglou/pretext";
import { renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import type { AssistantMessage } from "@/types/conversation/message/entity";

import { useRoundHeights } from "./use-message-height";

vi.mock("@chenglou/pretext", () => ({
  prepare: vi.fn((text: string) => text),
  layout: (text: string, width: number, lineHeight: number) => ({
    height: Math.ceil(text.length / width) * lineHeight,
  }),
}));

it("reuses text layout across tool updates and remeasures edits and width changes", () => {
  const message: AssistantMessage = {
    message_id: "m", session_key: "s", agent_id: "a", round_id: "r",
    timestamp: 0, role: "assistant", content: [{ type: "text", text: "abcd" }],
  };
  const { result, rerender } = renderHook(
    ({ content, width }) => useRoundHeights(
      ["r"],
      new Map([["r", [{ ...message, content }]]]),
      width,
      new Set(),
    ),
    { initialProps: { content: message.content, width: 4 } },
  );
  const original = result.current.get("r")!;
  rerender({
    content: [...message.content, { type: "tool_use", id: "t", name: "Read", input: {} }],
    width: 4,
  });
  expect(result.current.get("r")).toBeGreaterThan(original);
  expect(prepare).toHaveBeenCalledTimes(1);

  const edited: AssistantMessage["content"] = [{ type: "text", text: "wxyz" }];
  rerender({ content: edited, width: 4 });
  expect(result.current.get("r")).toBe(original);
  rerender({ content: edited, width: 2 });
  expect(result.current.get("r")).toBeGreaterThan(original);
  const measuredText = vi.mocked(prepare).mock.calls.map(([text]) => text);
  expect(measuredText).toEqual(["abcd", "wxyz", "wxyz"]);
});
