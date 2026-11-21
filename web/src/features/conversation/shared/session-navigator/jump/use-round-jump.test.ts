import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Message } from "@/types/conversation/message/entity";

import { getConversationRoundNavigationTarget } from "../../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../../timeline/timeline-model";
import type { SessionNavigationItem } from "../session-navigator-model";
import { useRoundJump } from "./use-round-jump";

describe("round jump transaction", () => {
  afterEach(() => vi.useRealTimers());

  it("reports the end of the jump only after the landing released the viewport", () => {
    vi.useFakeTimers({ toFake: ["requestAnimationFrame", "cancelAnimationFrame"] });
    const scrollElement = document.createElement("div");
    scrollElement.getBoundingClientRect = () => ({ bottom: 500, top: 0 }) as DOMRect;
    const round = document.createElement("div");
    round.dataset.conversationRoundId = "round-2";
    round.getBoundingClientRect = () => ({ bottom: 400, top: 100 }) as DOMRect;
    scrollElement.append(round);
    // 调用方在这一刻重新记录阅读锚点：必须晚于落点滚动和导航目标的释放。
    const targetsAtEnd: Array<string | null> = [];
    const onNavigateEnd = vi.fn(() => {
      targetsAtEnd.push(getConversationRoundNavigationTarget(scrollElement));
    });
    const { result } = renderHook(() => useRoundJump({
      activateRound: () => undefined,
      onNavigateEnd,
      roundScrollRef: { current: { scrollToRoundId: () => true } },
      scopeKey: "session",
      scrollRef: { current: scrollElement },
      timeline: {
        live_round_ids: [],
        message_groups: new Map([["round-2", [{ message_id: "user-2" } as Message]]]),
      } as unknown as ConversationTimeline,
    }));

    act(() => result.current.jumpToRound({ roundId: "round-2" } as SessionNavigationItem));
    expect(getConversationRoundNavigationTarget(scrollElement)).toBe("round-2");
    expect(onNavigateEnd).not.toHaveBeenCalled();

    act(() => { vi.advanceTimersToNextFrame(); });
    expect(targetsAtEnd).toEqual([null]);
  });
});
