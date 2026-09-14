// INPUT: 可控 RAF、历史正文、大块追加和 runtime 终态。
// OUTPUT: 长尾降低提交次数、终态及时排空，保留 grapheme 与快照修正语义。
// POS: 共享流式 hook 的集成时序回归，使用真实字符时钟与全局调度器。

import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { splitTextGraphemes } from "@/lib/text-graphemes";
import { useSmoothStreamingMarkdownState } from "./use-smooth-streaming-markdown-content";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it("合并长尾提交且终态排空，历史和修正直接对齐", () => {
  let timestamp = 0;
  let frameId = 0;
  const frames = new Map<number, FrameRequestCallback>();
  vi.spyOn(performance, "now").mockImplementation(() => timestamp);
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    frames.set(++frameId, callback);
    return frameId;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => frames.delete(id));
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  const step = () => {
    timestamp += 34;
    const callbacks = [...frames.values()];
    frames.clear();
    act(() => callbacks.forEach((callback) => callback(timestamp)));
  };
  const history = "历史".repeat(500);
  const content = history + "中文👩🏽‍💻e\u0301".repeat(500);
  const lengths = new Set<number>();
  let length = 0;
  for (const unit of splitTextGraphemes(content)) lengths.add(length += unit.length);
  const { result, rerender, unmount } = renderHook(
    (props) => useSmoothStreamingMarkdownState(props.content, props.enabled),
    { initialProps: { content: history, enabled: true } },
  );
  expect(result.current.content).toBe(history);
  rerender({ content, enabled: true });
  let commits = 0;
  let previous = history;
  for (let frame = 0; frame < 30; frame++) {
    step();
    const displayed = result.current.content;
    expect(content.startsWith(displayed)).toBe(true);
    expect(lengths.has(displayed.length)).toBe(true);
    if (displayed !== previous) commits++;
    previous = displayed;
  }
  expect(commits).toBeGreaterThan(5);
  expect(commits).toBeLessThanOrEqual(11);
  rerender({ content, enabled: false });
  for (let frame = 0; frame < 22; frame++) step();
  expect(result.current).toEqual({ content, isStreaming: false });
  expect(frames.size).toBe(0);
  rerender({ content: "修正后的完整正文", enabled: true });
  expect(result.current.content).toBe("修正后的完整正文");
  unmount();
  expect(frames.size).toBe(0);
});
