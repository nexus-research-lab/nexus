// INPUT: 全局反馈事实、关闭/恢复动作与消息生命周期变化。
// OUTPUT: 证明反馈恢复与关闭动作独立、正确播报和重置后的自动收起。
// POS: Feedback primitives DOM 行为测试；业务失败分类与请求重试仍由 feature 决定。

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
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

  it("restarts auto-dismiss timing when the visible message changes", () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    const view = render(
      <FeedbackTestProvider>
        <FeedbackBannerViewport item={{ message: "第一条消息", onDismiss, title: "已保存", tone: "info" }} />
      </FeedbackTestProvider>,
    );

    act(() => vi.advanceTimersByTime(3000));
    view.rerender(
      <FeedbackTestProvider>
        <FeedbackBannerViewport item={{ message: "第二条消息", onDismiss, title: "已同步", tone: "info" }} />
      </FeedbackTestProvider>,
    );
    act(() => vi.advanceTimersByTime(4999));
    expect(onDismiss).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(onDismiss).toHaveBeenCalledOnce();
    expect(screen.getByRole("status")).toBeTruthy();
    view.rerender(<FeedbackTestProvider><FeedbackBannerViewport item={null} /></FeedbackTestProvider>);
    expect(screen.queryByRole("status")).toBeNull();
  });
});
