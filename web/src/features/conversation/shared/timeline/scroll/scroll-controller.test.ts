import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getConversationScrollWriteConflicts,
  shiftConversationScrollTop,
  writeConversationScrollTop,
} from "./scroll-controller";

function nextFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

describe("conversation scroll controller", () => {
  afterEach(() => vi.restoreAllMocks());

  it("applies compensation on top of the live scrollTop", () => {
    const element = document.createElement("div");
    document.body.append(element);
    writeConversationScrollTop(element, "anchor", 120);
    // 用户在异步等待期间继续滚动。
    element.scrollTop = 300;
    expect(shiftConversationScrollTop(element, "anchor", 80)).toBe(380);
    element.remove();
  });

  it("records different owners writing in the same frame but not across frames", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const element = document.createElement("div");
    document.body.append(element);

    writeConversationScrollTop(element, "follow", 10);
    writeConversationScrollTop(element, "follow", 20);
    expect(getConversationScrollWriteConflicts(element)).toEqual([]);

    await nextFrame();
    writeConversationScrollTop(element, "anchor", 30);
    expect(getConversationScrollWriteConflicts(element)).toEqual([]);

    // 重申当前位置不移动视口，不算第二个写入者。
    writeConversationScrollTop(element, "session", 30);
    expect(getConversationScrollWriteConflicts(element)).toEqual([]);

    shiftConversationScrollTop(element, "virtualizer", 5);
    expect(getConversationScrollWriteConflicts(element)).toEqual([
      { first: "anchor", second: "virtualizer" },
    ]);
    element.remove();
  });
});
