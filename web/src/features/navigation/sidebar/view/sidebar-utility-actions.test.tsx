// INPUT: 侧栏折叠动作标签、可见状态与展开/收起命令。
// OUTPUT: 证明系统动作复用共享圆形 IconButton，并转发精确命令。
// POS: 侧栏底部动作 DOM 行为测试；路由和更新桥接由各自所有者负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SidebarPanelToggleAction } from "./sidebar-utility-actions";

describe("SidebarPanelToggleAction", () => {
  it("uses the shared round icon action and keeps panel commands distinct", async () => {
    const user = userEvent.setup();
    const onCollapse = vi.fn();
    const onExpand = vi.fn();
    render(
      <SidebarPanelToggleAction
        labels={{ collapse: "收起侧栏", expand: "展开侧栏" }}
        onCollapse={onCollapse}
        onExpand={onExpand}
        showPanelToggle
        variant="panel"
      />,
    );

    const action = screen.getByRole("button", { name: "收起侧栏" });
    expect(action.getAttribute("type")).toBe("button");
    expect(action.className).toContain("h-8 w-8");
    expect(action.className).toContain("rounded-full");

    await user.click(action);
    expect(onCollapse).toHaveBeenCalledOnce();
    expect(onExpand).not.toHaveBeenCalled();
  });
});
