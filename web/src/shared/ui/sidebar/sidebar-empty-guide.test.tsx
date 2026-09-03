// INPUT: 侧栏失败状态、恢复说明和可执行动作。
// OUTPUT: 证明紧凑排版、状态播报、说明去重与共享 Button 点击合同。
// POS: SidebarEmptyGuide DOM 行为测试；失败事实和动作选择由业务层负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { CircleAlert } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { SidebarEmptyGuide } from "./sidebar-empty-guide";

describe("SidebarEmptyGuide", () => {
  it("uses shared compact styles and keeps one actionable recovery path", () => {
    const onAction = vi.fn();
    const { container } = render(
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
    expect(guide.className).toContain("surface-radius-md");
    expect(screen.getByText("无法读取").className).toContain("ui-type-caption");
    expect(screen.getByText("会话目录暂时不可用。").className)
      .toContain("ui-type-tone-muted");
    expect(screen.queryByText("暂时没有会话。")).toBeNull();
    expect(screen.queryByText("网络恢复后重试。")).toBeNull();

    const action = screen.getByRole("button", { name: "重新加载" });
    expect(action.className).toContain("ui-type-caption");
    expect(action.className).toContain("radius-control-xs");
    fireEvent.click(action);
    expect(onAction).toHaveBeenCalledOnce();
    expect(container.querySelector("button")?.getAttribute("type")).toBe("button");
  });
});
