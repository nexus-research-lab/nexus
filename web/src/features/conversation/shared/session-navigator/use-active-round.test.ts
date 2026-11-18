import { renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { useActiveRound } from "./use-active-round";

afterEach(() => {
  vi.restoreAllMocks();
});

it("keeps one scroll subscription across streaming round updates and syncs in the next frame", () => {
  const scrollElement = document.createElement("div");
  const addListener = vi.spyOn(scrollElement, "addEventListener");
  const frames: FrameRequestCallback[] = [];
  vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
    frames.push(callback);
    return frames.length;
  });
  vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => undefined);
  const scrollRef = { current: scrollElement };

  const { rerender } = renderHook(
    ({ roundIds }) => useActiveRound({ roundIds, scopeKey: "scope", scrollRef }),
    { initialProps: { roundIds: ["r1", "r2"] } },
  );
  const scrollSubscriptions = () => addListener.mock.calls
    .filter(([type]) => type === "scroll").length;
  expect(scrollSubscriptions()).toBe(1);

  // 流式帧会传入内容相同或追加的新数组：不得重建订阅或在提交阶段同步读布局，
  // 只排入一次下一帧同步。
  rerender({ roundIds: ["r1", "r2"] });
  rerender({ roundIds: ["r1", "r2", "r3"] });
  expect(scrollSubscriptions()).toBe(1);
  expect(frames).toHaveLength(1);
});
