// INPUT: Mermaid 异步渲染替身与源码更新。
// OUTPUT: 清空源码不残留旧图，迟到结果不复活旧内容。
// POS: Mermaid 渲染生命周期回归。
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { useMermaidSvg } from "./use-mermaid-svg";

const mermaid = vi.hoisted(() => ({
  initialize: vi.fn(),
  parse: vi.fn(),
  render: vi.fn(),
}));
vi.mock("mermaid", () => ({ default: mermaid }));

beforeEach(() => {
  vi.useFakeTimers();
  mermaid.parse.mockResolvedValue(true);
  mermaid.render.mockResolvedValue({ svg: '<svg xmlns="http://www.w3.org/2000/svg"><text>Current</text></svg>' });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.resetAllMocks();
});

it("clears a previously rendered diagram when streaming source becomes empty", async () => {
  const { result, rerender } = renderHook(({ chart }) => useMermaidSvg(chart, true, "empty"), {
    initialProps: { chart: "graph TD; A-->B" },
  });
  await act(async () => { await vi.advanceTimersByTimeAsync(300); });
  expect(result.current.svg).toContain("Current");
  rerender({ chart: " \n" });
  expect(result.current).toEqual({ svg: "", error: null, is_rendering: false });
});

it("ignores a pending render that completes after the source was cleared", async () => {
  let finish!: (value: { svg: string }) => void;
  mermaid.render.mockReturnValue(new Promise<{ svg: string }>((resolve) => { finish = resolve; }));
  const { result, rerender } = renderHook(({ chart }) => useMermaidSvg(chart, true, "late"), {
    initialProps: { chart: "graph TD; A-->B" },
  });
  await act(async () => { await vi.advanceTimersByTimeAsync(300); });
  expect(mermaid.render).toHaveBeenCalledOnce();
  rerender({ chart: "" });
  await act(async () => { finish({ svg: '<svg xmlns="http://www.w3.org/2000/svg"><text>Old</text></svg>' }); });
  expect(result.current).toEqual({ svg: "", error: null, is_rendering: false });
});
