// INPUT: 行内提示的 tone、variant、文案、单一动作和 pending 状态。
// OUTPUT: 证明共享提示持有 surface/排版/Button 语义，并保持原生可访问行为。
// POS: UiInlineNotice DOM 行为测试；业务错误分类与恢复条件由 feature 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { RefreshCw, TriangleAlert } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";

describe("UiInlineNotice", () => {
  it("renders one shared contained notice and delegates its only action", () => {
    const onRefresh = vi.fn();
    render(
      <UiInlineNotice
        action={{
          icon: <RefreshCw />,
          label: "刷新",
          onClick: onRefresh,
        }}
        icon={<TriangleAlert />}
        message="当前消息结果仍需确认"
        title="投递结果未知"
        tone="warning"
      />,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(status.getAttribute("data-inline-notice-variant")).toBe("contained");
    expect(status.className).toContain("surface-radius-sm");
    expect(screen.getByText("投递结果未知").className).toContain("ui-type-metadata");
    expect(screen.getByText("当前消息结果仍需确认").className).toContain("ui-type-metadata");

    const action = screen.getByRole("button", { name: "刷新" });
    expect(action.getAttribute("type")).toBe("button");
    fireEvent.click(action);
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it("keeps a pending edge action disabled and exposes its busy state", () => {
    const onRefresh = vi.fn();
    render(
      <UiInlineNotice
        action={{
          icon: <RefreshCw />,
          label: "重新检查",
          onClick: onRefresh,
          pending: true,
        }}
        icon={<TriangleAlert />}
        message="上一次成功内容仍然可见"
        title="工作图暂时不可用"
        tone="warning"
        variant="edge"
      />,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-variant")).toBe("edge");
    expect(status.className).toContain("sm:flex-nowrap");
    const action = screen.getByRole("button", { name: "重新检查" });
    expect(action.hasAttribute("disabled")).toBe(true);
    expect(action.getAttribute("aria-busy")).toBe("true");
    expect(status.querySelector("[data-inline-notice-action-icon]")?.className)
      .toContain("animate-spin");
    fireEvent.click(action);
    expect(onRefresh).not.toHaveBeenCalled();
  });
});
