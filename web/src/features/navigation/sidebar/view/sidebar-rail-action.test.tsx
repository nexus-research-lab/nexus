// INPUT: 侧栏动作的主入口/固定会话布局、选中态与点击事件。
// OUTPUT: 证明两类入口共享图标几何、排版和无障碍状态。
// POS: SidebarRailAction DOM 行为合同；业务路由与拖放排序另行测试。

import { MessageCircle } from "lucide-react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SidebarRailAction } from "./sidebar-rail-action";

describe("SidebarRailAction", () => {
  it("shares one icon frame and caption recipe across primary and pinned entries", () => {
    const { rerender } = render(
      <SidebarRailAction
        active
        badgeCount={3}
        icon={MessageCircle}
        label="聊天"
        layout="primary"
      />,
    );

    const primary = screen.getByRole("button", { name: "聊天" });
    expect(primary.getAttribute("aria-current")).toBe("page");
    expect(primary.getAttribute("aria-pressed")).toBe("true");
    expect(primary.className).toContain("h-[50px]");
    expect(primary.className).toContain("ui-type-caption");
    expect(primary.querySelector("svg")?.parentElement?.className).toContain("h-8 w-8");
    expect(primary.querySelector("svg")?.getAttribute("class")).toContain("h-[18px]");
    expect(screen.getByText("3")).not.toBeNull();

    rerender(
      <SidebarRailAction
        active={false}
        icon={MessageCircle}
        label="项目讨论"
        layout="pinned"
        supplementalLabel="拖动排序"
      />,
    );

    const pinned = screen.getByRole("button", { name: /项目讨论/ });
    expect(pinned.getAttribute("aria-current")).toBeNull();
    expect(pinned.getAttribute("aria-pressed")).toBeNull();
    expect(pinned.className).toContain("absolute inset-0");
    expect(pinned.className).toContain("ui-type-caption");
    expect(pinned.querySelector("svg")?.parentElement?.className).toContain("h-8 w-8");
    expect(screen.getByText("拖动排序").className).toContain("sr-only");
  });

  it("shows a pinned avatar and keeps its title available without a visible caption", async () => {
    render(<SidebarRailAction active={false} layout="pinned" label="每周工作回顾"
      iconContent={<img alt="" src="/avatar.png" />} />);
    const button = screen.getByRole("button", { name: "每周工作回顾" });
    expect(button.querySelector("img")?.getAttribute("src")).toBe("/avatar.png");
    expect(screen.getByText("每周工作回顾").className).toBe("sr-only");
    await userEvent.setup().hover(button);
    expect(await screen.findByRole("tooltip")).toBeTruthy();
  });

  it("forwards its native button action", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <SidebarRailAction
        active={false}
        icon={MessageCircle}
        label="联系人"
        layout="primary"
        onClick={onClick}
      />,
    );

    await user.click(screen.getByRole("button", { name: "联系人" }));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
