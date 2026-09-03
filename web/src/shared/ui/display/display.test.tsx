// INPUT: Badge/Counter/ResourceState 的有限状态、动作与用户事件。
// OUTPUT: 证明标记边界、live-region 语义、忙碌互斥和恢复动作呈现合同。
// POS: Display primitives DOM 行为测试；失败分类和业务恢复策略由 feature 决定。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";

describe("display badges", () => {
  it("renders an optional semantic dot and caps positive counters", () => {
    const { container, rerender } = render(
      <>
        <UiBadge showDot tone="running">运行中</UiBadge>
        <UiCounterBadge count={120} max={99} />
      </>,
    );

    expect(screen.getByText("运行中").querySelector(".bg-current")).toBeTruthy();
    expect(screen.getByText("99+")).toBeTruthy();

    rerender(<UiCounterBadge count={0} />);
    expect(container.textContent).toBe("");
  });
});

describe("UiResourceState", () => {
  it("exposes loading as a busy polite status", () => {
    render(<UiResourceState state="loading" title="正在加载" />);
    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(status.getAttribute("aria-live")).toBe("polite");
  });

  it("shows one safe recovery action without duplicating its next-step copy", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <UiResourceState
        impact="当前列表不可用"
        nextStep="请重试"
        primaryAction={{ label: "重试", onClick: onRetry }}
        state="error"
        title="加载失败"
        urgency="assertive"
      />,
    );

    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.getByText("当前列表不可用")).toBeTruthy();
    expect(screen.queryByText("请重试")).toBeNull();
    await user.click(screen.getByRole("button", { name: "重试" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("locks a busy action and replaces its label", () => {
    render(
      <UiResourceState
        primaryAction={{
          busy: true,
          busyLabel: "保存中",
          label: "保存",
          onClick: vi.fn(),
        }}
        state="success"
        title="可以保存"
      />,
    );

    const action = screen.getByRole("button", { name: "保存中" });
    expect((action as HTMLButtonElement).disabled).toBe(true);
    expect(action.getAttribute("aria-busy")).toBe("true");
    expect(screen.queryByText("保存", { exact: true })).toBeNull();
  });
});
