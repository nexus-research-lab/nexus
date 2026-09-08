// INPUT: 能力导航项、选中状态与选择回调。
// OUTPUT: 证明能力行复用 ListRow、语义形状与 Typography，并保持鼠标和键盘选择。
// POS: 能力侧栏行 DOM 合同；摘要读取和路由写入由 panel/controller 负责。

import { Puzzle } from "lucide-react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { CapabilitySidebarItem } from "./capability-sidebar-model";
import { CapabilitySidebarItemView } from "./capability-sidebar-item";

const ITEM = {
  icon: Puzzle,
  id: "skills",
  label: "技能",
  meta: "30",
  path: "/capability/skills",
} satisfies CapabilitySidebarItem;

describe("CapabilitySidebarItemView", () => {
  it("uses shared row semantics and dispatches the exact item", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const { container } = render(
      <CapabilitySidebarItemView
        active
        item={ITEM}
        onSelect={onSelect}
      />,
    );

    const row = screen.getByRole("button");
    expect(row.className).toContain("radius-control-md");
    expect(container.querySelector("span.radius-control-md")).toBeTruthy();
    expect(screen.getByText(ITEM.label).className).toContain("ui-type-section-title");
    expect(screen.getByText(ITEM.meta).className).toContain("ui-type-caption");

    await user.click(row);
    expect(onSelect).toHaveBeenCalledWith(ITEM);
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledTimes(2);
  });
});
