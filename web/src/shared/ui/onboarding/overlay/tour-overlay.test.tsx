// INPUT: 无锚点的居中 Tour step 与运行中发生的浏览器视口变化。
// OUTPUT: 证明共享 Overlay 在窗口缩放后重新约束卡片，而不是保留旧视口坐标。
// POS: Onboarding Tour Portal/定位行为测试；卡片内容与纯几何公式由各自测试负责。

import { fireEvent, render } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { OnboardingTourOverlay } from "./tour-overlay";



it("respects consumed Escape and IME cancellation before closing on ordinary Escape", () => {
  const onClose = vi.fn();
  const view = render(
    <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <input aria-label="Editor" onKeyDown={(event) => {
        if (event.key === "Escape") event.preventDefault();
      }} />
      <OnboardingTourOverlay
        onClose={onClose} onNext={vi.fn()} onPrevious={vi.fn()} stepIndex={0}
        tour={{ id: "escape-tour", steps: [{ id: "first", title: "Guide", description: "Help", placement: "center" }] }}
      />
    </I18N_CONTEXT.Provider>,
  );
  fireEvent.keyDown(view.getByRole("textbox", { name: "Editor" }), { key: "Escape" });
  fireEvent.keyDown(document, { key: "Escape", isComposing: true });
  fireEvent.keyDown(document, { key: "Escape", keyCode: 229 });
  fireEvent.keyDown(document, { key: "Escape", which: 229 });
  expect(onClose).not.toHaveBeenCalled();
  fireEvent.keyDown(document, { key: "Escape" });
  expect(onClose).toHaveBeenCalledOnce();
  view.unmount();
  fireEvent.keyDown(document, { key: "Escape" });
  expect(onClose).toHaveBeenCalledOnce();
});
