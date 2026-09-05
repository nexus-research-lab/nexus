// INPUT: 剪贴板受控结果、反馈时长与组件卸载。
// OUTPUT: 证明复制反馈按时归零，并且卸载释放计时器且不接收迟到反馈。
// POS: React 剪贴板反馈生命周期测试；原生回退由 browser/clipboard 测试覆盖。
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { writeTextToClipboard } from "../browser/clipboard";
import { useCopyToClipboard } from "./use-copy-to-clipboard";

vi.mock("../browser/clipboard", () => ({ writeTextToClipboard: vi.fn() }));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.resetAllMocks();
});

describe("copy feedback lifecycle", () => {
  it("restarts one feedback timer for repeated successful copies and releases it on unmount", async () => {
    vi.useFakeTimers();
    vi.mocked(writeTextToClipboard).mockResolvedValue(true);
    const { result, unmount } = renderHook(() => useCopyToClipboard({ feedback_timeout_ms: 200 }));
    await act(async () => { await result.current.copy("第一段"); });
    expect(result.current.copied).toBe(true);
    act(() => { vi.advanceTimersByTime(100); });
    await act(async () => { await result.current.copy("第二段"); });
    expect(vi.getTimerCount()).toBe(1);
    act(() => { vi.advanceTimersByTime(199); });
    expect(result.current.copied).toBe(true);
    act(() => { vi.advanceTimersByTime(1); });
    expect(result.current.copied).toBe(false);
    await act(async () => { await result.current.copy("第三段"); });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("returns a pending native copy result without scheduling feedback after unmount", async () => {
    vi.useFakeTimers();
    let resolveCopy!: (success: boolean) => void;
    vi.mocked(writeTextToClipboard).mockReturnValue(new Promise((resolve) => { resolveCopy = resolve; }));
    const { result, unmount } = renderHook(() => useCopyToClipboard());
    const pending = result.current.copy("已请求的文本");
    unmount();
    await act(async () => {
      resolveCopy(true);
      expect(await pending).toBe(true);
    });
    expect(vi.getTimerCount()).toBe(0);
  });
});
