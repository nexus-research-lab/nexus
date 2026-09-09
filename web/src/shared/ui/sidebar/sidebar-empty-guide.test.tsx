// INPUT: 侧栏失败状态、恢复说明和可执行动作。
// OUTPUT: 证明状态播报、说明优先级与恢复动作。
// POS: SidebarEmptyGuide DOM 行为测试；失败事实和动作选择由业务层负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { CircleAlert } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { SidebarEmptyGuide } from "./sidebar-empty-guide";

describe("SidebarEmptyGuide", () => {
  it("keeps one actionable recovery path", () => {
    const onAction = vi.fn();
    render(
      <SidebarEmptyGuide
        actionLabel="重新加载"
        description="暂时没有会话。"
        icon={CircleAlert}
        impact="会话目录暂时不可用。"
        nextStep="网络恢复后重试。"
        onAction={onAction}
        title="无法读取"
      />,
    );

    const guide = screen.getByRole("status");
    expect(guide.textContent).toContain("无法读取");
    expect(guide.textContent).toContain("会话目录暂时不可用。");
    expect(screen.queryByText("暂时没有会话。")).toBeNull();
    expect(screen.queryByText("网络恢复后重试。")).toBeNull();

    const action = screen.getByRole("button", { name: "重新加载" });
    fireEvent.click(action);
    expect(onAction).toHaveBeenCalledOnce();
    expect(action.getAttribute("type")).toBe("button");
  });
});
