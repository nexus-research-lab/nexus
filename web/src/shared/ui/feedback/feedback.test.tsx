// INPUT: 全局反馈事实、关闭/恢复动作与消息生命周期变化。
// OUTPUT: 证明反馈使用语义 surface/layer、正确播报、单一动作和重置后的自动收起。
// POS: Feedback primitives DOM 行为测试；业务失败分类与请求重试仍由 feature 决定。

import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";

function FeedbackTestProvider({ children }: { children: ReactNode }) {
  return (
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key === "common.close" ? "关闭" : key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>
  );
}

afterEach(() => {
  vi.useRealTimers();
});

describe("FeedbackBanner", () => {
  it("keeps one recovery action and an independently reachable dismiss action", () => {
    const onRetry = vi.fn();
    const onDismiss = vi.fn();
    render(
      <FeedbackTestProvider>
        <FeedbackBanner
          action={{ label: "重试", onClick: onRetry }}
          impact="当前列表不可用"
          nextStep="请重新加载"
          onDismiss={onDismiss}
          title="加载失败"
          tone="error"
          urgency="assertive"
        />
      </FeedbackTestProvider>,
    );

    const alert = screen.getByRole("alert");
    expect(alert.className).toContain("surface-popover");
    expect(alert.className).toContain("surface-radius-md");
    expect(alert.querySelector("svg")?.getAttribute("class")).toContain("ui-type-tone-danger");
    expect(screen.getByText("当前列表不可用")).toBeTruthy();
    expect(screen.queryByText("请重新加载")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("restarts auto-dismiss timing when the visible message changes", () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    const view = render(
      <FeedbackTestProvider>
        <FeedbackBanner message="第一条消息" onDismiss={onDismiss} title="已保存" tone="info" />
      </FeedbackTestProvider>,
    );

    act(() => vi.advanceTimersByTime(3000));
    view.rerender(
      <FeedbackTestProvider>
        <FeedbackBanner message="第二条消息" onDismiss={onDismiss} title="已同步" tone="info" />
      </FeedbackTestProvider>,
    );
    act(() => vi.advanceTimersByTime(4999));
    expect(onDismiss).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("projects each feedback tone through the shared foreground recipe", () => {
    const { container } = render(
      <FeedbackTestProvider>
        <FeedbackBanner message="同步完成" title="已同步" tone="success" />
      </FeedbackTestProvider>,
    );

    expect(container.querySelector("[role=status] svg")?.getAttribute("class"))
      .toContain("ui-type-tone-success");
  });

  it("places the floating viewport on its named layer", () => {
    const { container } = render(
      <FeedbackTestProvider>
        <FeedbackBannerViewport
          item={{ message: "设置已经保存", title: "已保存", tone: "success" }}
        />
      </FeedbackTestProvider>,
    );

    expect(container.querySelector("[data-feedback-viewport]")?.className).toContain("ui-layer-feedback");
    expect(screen.getByRole("status")).toBeTruthy();
  });
});
