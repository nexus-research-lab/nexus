import { describe, expect, it } from "vitest";

import {
  resolveConversationVirtualHandoff,
  scrollConversationVirtualElement,
  shouldAdjustConversationVirtualScrollPosition,
} from "./use-conversation-virtual-scroll-policy";

describe("static to virtual feed handoff", () => {
  const round = (index: number, top: number, bottom: number) => ({
    dataset: { conversationRoundIndex: String(index) },
    getBoundingClientRect: () => ({ bottom, top }),
  });
  const viewport = (rounds: ReturnType<typeof round>[]) => ({
    getBoundingClientRect: () => ({ top: 100 }),
    querySelectorAll: () => rounds,
    scrollTop: 1_380,
  }) as unknown as HTMLElement;
  // 真实高度 480/480/420，估高统一 60：静态 scrollTop 1380 落在估高区间之外。
  const rounds = [round(0, -1_280, -800), round(1, -800, -320), round(2, -320, 100), round(3, 100, 520)];

  it("starts the Virtualizer inside the round being read instead of at the static scrollTop", () => {
    expect(resolveConversationVirtualHandoff(viewport(rounds), () => 60, true)).toEqual({
      initialOffset: 180,
      readingAnchor: { index: 3, offsetTop: 0 },
    });
    // 已读入该轮 300px，但估高只有 60：初始偏移仍留在该轮的估高范围内。
    const scrolledInto = [...rounds.slice(0, 2), round(2, -620, -200), round(3, -200, 220)];
    expect(resolveConversationVirtualHandoff(viewport(scrolledInto), () => 60, true)).toEqual({
      initialOffset: 239,
      readingAnchor: { index: 3, offsetTop: -300 },
    });
  });

  it("inherits the scrollTop when FOLLOW owns the position", () => {
    expect(resolveConversationVirtualHandoff(viewport(rounds), () => 60, false)).toEqual({
      initialOffset: 1_380,
      readingAnchor: null,
    });
  });
});

describe("virtual conversation size changes", () => {
  const instance = { scrollOffset: 600, scrollRect: { height: 400 }, getTotalSize: () => 1000 };
  const following = { bottomScrollActive: false, followingLatest: true, userScrollActive: false };

  it("keeps the bottom anchored when a live process grows or collapses", () => {
    for (const delta of [-300, 200]) {
      expect(shouldAdjustConversationVirtualScrollPosition({ end: 1000 }, delta, instance, following)).toBe(true);
    }
  });

  it("leaves sizing to an active bottom or navigation transaction", () => {
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 1000 }, -300, instance, { ...following, bottomScrollActive: true })).toBe(false);
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 1000 }, -300, instance, { ...following, navigationActive: true })).toBe(false);
  });

  it("preserves reading above changing content and respects direct user input", () => {
    const reading = { ...following, followingLatest: false };
    const scrolled = { ...instance, scrollOffset: 300 };
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 200 }, -100, scrolled, reading)).toBe(true);
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 900 }, -100, scrolled, reading)).toBe(false);
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 900 }, -100, scrolled, { ...reading, userScrollActive: true })).toBe(false);
  });

  it("keeps compensating rounds above the viewport while the user scrolls up into loaded history", () => {
    const reading = { ...following, followingLatest: false, userScrollActive: true };
    const scrolled = { ...instance, scrollOffset: 300 };
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 200 }, 900, scrolled, reading)).toBe(true);
  });

  it("judges rows by the reading anchor while a window topology change commits", () => {
    // 前插 3 轮后锚点从下标 2 移到 5；scrollOffset 仍是旧拓扑的位置，不能再用来判断。
    const reading = { ...following, followingLatest: false, topologyAnchorIndex: 5, userScrollActive: true };
    const stale = { ...instance, scrollOffset: 300 };
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 900, index: 4 }, 160, stale, reading)).toBe(true);
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 200, index: 5 }, 160, stale, reading)).toBe(false);
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 200, index: 0 }, 160, stale, { ...reading, topologyAnchorIndex: 0 })).toBe(false);
  });

  it("applies each measured delta once on top of the live scrollTop", () => {
    const element = { scrollTop: 240, scrollTo: () => { throw new Error("absolute write"); } };
    const virtualizer = {
      ...instance,
      scrollElement: element as unknown as Element,
      scrollOffset: 300,
    };
    // 两次 scroll 事件之间连续测量两个轮次：TanStack 传入的是累计位移 30、90。
    let accumulated = 0;
    for (const delta of [30, 60]) {
      accumulated += delta;
      expect(shouldAdjustConversationVirtualScrollPosition({ end: 200 }, delta, virtualizer, {
        ...following,
        followingLatest: false,
      })).toBe(true);
      scrollConversationVirtualElement(300, { adjustments: accumulated }, virtualizer);
    }
    expect(element.scrollTop).toBe(330);

    let target: ScrollToOptions | null = null;
    const positioned = { scrollTop: 0, scrollTo: (options: ScrollToOptions) => { target = options; } };
    scrollConversationVirtualElement(500, { behavior: "smooth" }, {
      scrollElement: positioned as unknown as Element,
      scrollOffset: 0,
    });
    expect(target).toEqual({ behavior: "smooth", top: 500 });
  });
});
