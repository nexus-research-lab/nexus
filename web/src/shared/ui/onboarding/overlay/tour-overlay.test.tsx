// INPUT: 无锚点的居中 Tour step 与运行中发生的浏览器视口变化。
// OUTPUT: 证明共享 Overlay 在窗口缩放后重新约束卡片，而不是保留旧视口坐标。
// POS: Onboarding Tour Portal/定位行为测试；卡片内容与纯几何公式由各自测试负责。

import { act, fireEvent, render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { OnboardingTourOverlay } from "./tour-overlay";

function setViewport(width: number, height: number): void {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
  Object.defineProperty(window, "innerHeight", { configurable: true, value: height });
}

describe("OnboardingTourOverlay", () => {
  it("repositions a centered step when the window becomes narrow", async () => {
    const originalWidth = window.innerWidth;
    const originalHeight = window.innerHeight;
    setViewport(1440, 900);

    try {
      render(
        <I18N_CONTEXT.Provider
          value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
        >
          <OnboardingTourOverlay
            onClose={vi.fn()}
            onNext={vi.fn()}
            onPrevious={vi.fn()}
            stepIndex={0}
            tour={{
              id: "responsive-tour",
              steps: [{
                description: "共享浮层必须留在当前窗口内。",
                id: "center",
                placement: "center",
                title: "响应式导览",
              }],
            }}
          />
        </I18N_CONTEXT.Provider>,
      );

      const card = document.querySelector<HTMLElement>("[data-onboarding-tour-card]");
      expect(card).not.toBeNull();
      const wideLeft = Number.parseFloat(card?.style.left ?? "0");

      act(() => {
        setViewport(720, 820);
        fireEvent(window, new Event("resize"));
      });

      await waitFor(() => {
        expect(Number.parseFloat(card?.style.left ?? "0")).toBeLessThan(wideLeft);
        expect(Number.parseFloat(card?.style.left ?? "0") + 344).toBeLessThanOrEqual(704);
      });
    } finally {
      setViewport(originalWidth, originalHeight);
    }
  });
});
