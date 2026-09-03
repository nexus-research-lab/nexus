// INPUT: 窄窗二级页标题、返回动作和页面动作挂载点。
// OUTPUT: 证明应用页头复用共享几何、排版与按钮原语并保留返回行为。
// POS: App 窄窗页头 DOM 合同；路由模式选择由 app-layout 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { MobileAppPageHeader } from "./mobile-app-page-header";

describe("MobileAppPageHeader", () => {
  it("uses the shared shell geometry and keeps the back action interactive", () => {
    const onBack = vi.fn();
    const { container } = render(
      <I18nProvider>
        <MobileAppPageHeader onBack={onBack} title="连接器" />
      </I18nProvider>,
    );

    const header = container.querySelector("header");
    const heading = screen.getByRole("heading", { name: "连接器" });
    const back = screen.getByRole("button", { name: /back|返回/i });

    expect(header?.querySelector("[data-desktop-window-controls-leading]")?.className)
      .toContain("h-[var(--mobile-shell-header-height,52px)]");
    expect(heading.className).toContain("ui-type-section-title");
    expect(back.className).toContain("radius-control-lg");

    fireEvent.click(back);
    expect(onBack).toHaveBeenCalledOnce();
  });
});
