// INPUT: Badge/Counter/ResourceState/Skeleton 的有限状态、动作与用户事件。
// OUTPUT: 证明标记边界、live-region 语义、骨架视觉层级、忙碌互斥和恢复动作呈现合同。
// POS: Display primitives DOM 行为测试；失败分类和业务恢复策略由 feature 决定。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";

import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiSkeleton, UiSkeletonCardList } from "@/shared/ui/display/skeleton";
import { UiStateBlock } from "@/shared/ui/display/state-block";

describe("display badges", () => {
  it("renders an optional semantic dot and caps positive counters", () => {
    const { container, rerender } = render(
      <>
        <UiBadge showDot tone="running">运行中</UiBadge>
        <UiBadge shape="pill" tone="idle">12</UiBadge>
        <UiCounterBadge count={120} max={99} />
      </>,
    );

    expect(screen.getByText("运行中").querySelector(".bg-current")).toBeTruthy();
    expect(screen.getByText("12").className).toContain("rounded-full");
    expect(screen.getByText("运行中").className).toContain("radius-control-xs");
    expect(screen.getByText("99+")).toBeTruthy();

    rerender(<UiCounterBadge count={0} />);
    expect(container.textContent).toBe("");
  });
});

describe("UiSkeleton", () => {
  it("owns semantic tone and reduced-motion-safe animation recipes", () => {
    const { rerender } = render(<UiSkeleton data-testid="skeleton" tone="strong" />);
    const skeleton = screen.getByTestId("skeleton");

    expect(skeleton.getAttribute("aria-hidden")).toBe("true");
    expect(skeleton.className.split(" ")).toContain("motion-safe:animate-pulse");
    expect(skeleton.className).toContain("_76%");

    rerender(<UiSkeleton data-testid="skeleton" tone="subtle" />);
    expect(skeleton.className).toContain("_48%");
  });
});

describe("UiSkeletonCardList", () => {
  it("announces one localized loading state for all decorative cards", () => {
    const view = (locale: "en" | "zh") => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
      <UiSkeletonCardList count={3} />
    </I18N_CONTEXT.Provider>;
    const { container, rerender } = render(view("en"));
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.getAllByText("Loading…")).toHaveLength(1);
    expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(3);
    rerender(view("zh"));
    expect(screen.queryByText("Loading…")).toBeNull();
    expect(screen.getAllByText(MESSAGES.zh["common.loading"])).toHaveLength(1);
  });
});

describe("UiQRCode", () => {
  it("uses the shared surface, shape, and code typography contracts", () => {
    const payload = "data:image/png;base64,AAAA";
    const { container } = render(
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => MESSAGES.zh[key] }}>
        <UiQRCode alt="授权二维码" payload={payload} />
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector("section.surface-radius-md")).toBeTruthy();
    expect(screen.getByRole("img", { name: "授权二维码" }).className)
      .toContain("surface-radius-sm");
    expect(screen.getByText(payload).className).toContain("ui-type-code");
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

  it("keeps both decision commands distinct while one action is busy", async () => {
    const user = userEvent.setup();
    const onKeep = vi.fn();
    const onReplace = vi.fn();
    const node = (busy: boolean) => <UiResourceState
      impact="The current document changed while this draft was open."
      nextStep="Choose a version."
      primaryAction={{ busy, busyLabel: "Applying the draft", label: "Use the current draft", onClick: onReplace }}
      secondaryAction={{ label: "Keep the saved version", onClick: onKeep }}
      size="sm"
      state="decision"
      title="Choose which document version to keep"
      tone="warning"
    />;
    const { rerender } = render(node(true));
    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.queryByText("Choose a version.")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Applying the draft" }));
    expect(onReplace).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Keep the saved version" }));
    expect(onKeep).toHaveBeenCalledOnce();
    rerender(node(false));
    await user.click(screen.getByRole("button", { name: "Use the current draft" }));
    expect(onReplace).toHaveBeenCalledOnce();
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

describe("UiStateBlock", () => {
  it("projects empty and compact states through semantic type and shape roles", () => {
    const { rerender } = render(
      <UiStateBlock
        description="创建一个对象后会显示在这里"
        icon={<span aria-hidden="true">+</span>}
        title="暂无对象"
      />,
    );

    expect(screen.getByRole("heading", { name: "暂无对象" }).className).toContain("ui-type-object-title");
    expect(screen.getByText("创建一个对象后会显示在这里").className).toContain("ui-type-supporting");
    expect(screen.getByText("+").parentElement?.className).toContain("surface-radius-md");

    rerender(
      <UiStateBlock
        description="请稍后重试"
        icon={<span aria-hidden="true">!</span>}
        size="sm"
        title="读取失败"
        tone="danger"
      />,
    );
    expect(screen.getByRole("heading", { name: "读取失败" }).className).toContain("ui-type-section-title");
    expect(screen.getByText("!").parentElement?.className).toContain("radius-control-md");
  });
});
