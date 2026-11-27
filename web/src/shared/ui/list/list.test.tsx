// INPUT: 列表行、嵌套动作、分组名称与用户键盘/点击事件。
// OUTPUT: 行启停、次动作隔离与可访问分组名称。
// POS: 列表交互的唯一测试入口；按钮通用合同由 Button 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiListSectionDivider } from "@/shared/ui/list/list-section-divider";

describe("UiListSectionDivider", () => {
  it("uses visible group names by default and preserves explicit naming across updates", () => {
    const { rerender } = render(<>
      <UiListSectionDivider label="Active conversations" />
      <UiListSectionDivider aria-label="Pinned" label="Current selection" />
    </>);
    const named = screen.getByRole("separator", { name: "Active conversations" });
    expect(named.getAttribute("aria-orientation")).toBe("horizontal");
    expect(named.getAttribute("aria-labelledby")).toBe(screen.getByText("Active conversations").id);
    expect(screen.getByRole("separator", { name: "Pinned" }).hasAttribute("aria-labelledby")).toBe(false);
    rerender(<>
      <span id="external-group-name">Saved conversations</span>
      <UiListSectionDivider aria-labelledby="external-group-name" label="Archived" />
      <UiListSectionDivider />
    </>);
    expect(screen.getByRole("separator", { name: "Saved conversations" })).toBeTruthy();
    const unnamed = screen.getByRole("separator", { name: "" });
    expect(unnamed.hasAttribute("aria-labelledby")).toBe(false);
  });

});
