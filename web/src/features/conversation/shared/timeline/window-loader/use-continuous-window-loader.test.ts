import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  clearConversationRoundNavigationTarget,
  setConversationRoundNavigationTarget,
} from "../scroll/round-scroll";
import { useContinuousWindowLoader } from "./use-continuous-window-loader";

describe("continuous window loader", () => {
  afterEach(() => vi.useRealTimers());

  it("re-evaluates the window edge once a navigator jump has landed, without waiting for a scroll event", async () => {
    vi.useFakeTimers({ toFake: ["requestAnimationFrame", "cancelAnimationFrame"] });
    const container = document.createElement("div");
    setConversationRoundNavigationTarget(container, "round-9");
    const loadNewerMessages = vi.fn(async () => true);
    renderHook(() => useContinuousWindowLoader({
      enabled: true,
      feedRef: { current: null },
      // 跳转页的末尾先被记为断点；它是否就是最新一轮要靠向后扩展一次才能确认。
      gapAfterRoundId: "round-9",
      hasOlderHistory: false,
      isBottomScrollActive: () => false,
      isFollowingLatest: () => false,
      isSessionLoading: false,
      loadNewerMessages,
      loadOlderMessages: async () => false,
      returnToLatestMessages: async () => false,
      revision: "round-8:round-9",
      scopeKey: "session",
      scrollRef: { current: container },
      setEvictionEdge: () => undefined,
      setWindowChangePending: () => undefined,
      windowRoundIds: ["round-8", "round-9"],
    }));

    await act(async () => { vi.advanceTimersToNextFrame(); });
    expect(loadNewerMessages).not.toHaveBeenCalled();

    // 落点确认后导航目标被释放，期间没有任何 scroll 事件。
    clearConversationRoundNavigationTarget(container);
    await act(async () => { vi.advanceTimersToNextFrame(); });
    expect(loadNewerMessages).toHaveBeenCalledOnce();
  });
});
