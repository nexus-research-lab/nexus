import { describe, expect, it } from "vitest";

import { shouldAdjustConversationVirtualScrollPosition } from "./use-conversation-virtual-scroll-policy";

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
    expect(shouldAdjustConversationVirtualScrollPosition({ end: 200 }, -100, scrolled, { ...reading, userScrollActive: true })).toBe(false);
  });
});
