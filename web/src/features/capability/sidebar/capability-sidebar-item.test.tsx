// INPUT: 能力导航项、选中状态与选择回调。
// OUTPUT: 证明能力行只显示名称，并通过鼠标和键盘选择精确条目。
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
  path: "/capability/skills",
} satisfies CapabilitySidebarItem;

describe("CapabilitySidebarItemView", () => {
  it("dispatches the exact item through mouse and keyboard", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <CapabilitySidebarItemView
        active
        item={ITEM}
        onSelect={onSelect}
      />,
    );

    const row = screen.getByRole("button");
    screen.getByText(ITEM.label);
    expect(row.textContent).toBe(ITEM.label);

    await user.click(row);
    expect(onSelect).toHaveBeenCalledWith(ITEM);
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledTimes(2);
  });
});
