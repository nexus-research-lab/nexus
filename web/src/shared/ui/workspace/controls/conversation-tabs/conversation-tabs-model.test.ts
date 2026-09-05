// INPUT: 纯标签身份、活动项、固定边缘动作和容器宽度。
// OUTPUT: 验证提取后的宽度模型仍填满可用轨道并在最小可读宽度进入溢出。
// POS: 共享标签几何回归；不依赖 Room 协议或持久化。

import { describe, expect, it } from "vitest";

import { calculateConversationTabWidths, hasConversationTabsOverflow } from "./conversation-tabs-model";

describe("conversation tab geometry", () => {
  it("fills the track after reserving both fixed actions, insets and tab gaps", () => {
    const widths = calculateConversationTabWidths({
      activeConversationId: "b", hasCreateButton: true, hasLeadingControl: true,
      hasTabsOverflow: false, tabs: [{ id: "a" }, { id: "b" }, { id: "c" }], trackWidth: 700,
    });
    expect([...widths.values()].reduce((sum, width) => sum + width, 0)).toBeCloseTo(700 - 64 - 8 - 4);
    expect(widths.get("b")!).toBeGreaterThan(widths.get("a")!);
    expect(widths.get("a")).toBe(widths.get("c"));
  });

  it("switches to readable overflow exactly at the stable width threshold", () => {
    const options = { conversationCount: 3, hasCreateButton: true, hasLeadingControl: true };
    expect(hasConversationTabsOverflow({ ...options, trackWidth: 440 })).toBe(false);
    expect(hasConversationTabsOverflow({ ...options, trackWidth: 439 })).toBe(true);
    const widths = calculateConversationTabWidths({
      ...options, activeConversationId: "b", hasTabsOverflow: true,
      tabs: [{ id: "a" }, { id: "b" }, { id: "c" }], trackWidth: 320,
    });
    expect(widths.get("b")!).toBeGreaterThanOrEqual(156);
    expect(widths.get("a")!).toBeGreaterThanOrEqual(104);
  });
});
