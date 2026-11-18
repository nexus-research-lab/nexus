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

it("measures a round once it stops growing and releases rounds that leave the feed", () => {
  vi.mocked(prepare).mockClear();
  const round = (id: string, text: string): AssistantMessage => ({
    message_id: `m-${id}`, session_key: "s", agent_id: "a", round_id: id,
    timestamp: 0, role: "assistant", content: [{ type: "text", text }],
  });
  const groups = new Map([["a", [round("a", "aaaa")]], ["b", [round("b", "bbbb")]]]);
  const { result, rerender } = renderHook(
    ({ ids, active }) => useRoundHeights(ids, groups, 4, active),
    { initialProps: { ids: ["a", "b"], active: new Set(["b"]) } },
  );
  // 活跃轮次只给固定基线，不测量正文。
  expect(vi.mocked(prepare).mock.calls.map(([text]) => text)).toEqual(["aaaa"]);

  rerender({ ids: ["a", "b"], active: new Set<string>() });
  expect(result.current.get("b")).toBe(result.current.get("a"));
  expect(vi.mocked(prepare).mock.calls.map(([text]) => text)).toEqual(["aaaa", "bbbb"]);

  // "a" 移出加载窗口后缓存随之释放，重新进入时重新测量。
  rerender({ ids: ["b"], active: new Set<string>() });
  rerender({ ids: ["a", "b"], active: new Set<string>() });
  expect(vi.mocked(prepare).mock.calls.map(([text]) => text)).toEqual(["aaaa", "bbbb", "aaaa"]);
});

it("falls back to the default height before the column is measured", () => {
  vi.mocked(prepare).mockClear();
  const message: AssistantMessage = {
    message_id: "m", session_key: "s", agent_id: "a", round_id: "r",
    timestamp: 0, role: "assistant", content: [{ type: "text", text: "abcd" }],
  };
  const { result } = renderHook(() => useRoundHeights(
    ["r"],
    new Map([["r", [message]]]),
    0,
    new Set(),
  ));
  expect(result.current.get("r")).toBe(200);
  expect(prepare).not.toHaveBeenCalled();
});
