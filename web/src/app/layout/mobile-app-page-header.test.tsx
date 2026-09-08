// INPUT: 窄窗二级页标题、返回动作和页面动作挂载点。
// OUTPUT: 证明应用页头暴露可访问标题并保留返回行为。
// POS: App 窄窗页头 DOM 合同；路由模式选择由 app-layout 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { MobileAppPageHeader } from "./mobile-app-page-header";

describe("MobileAppPageHeader", () => {
  it("keeps the accessible title and back action interactive", () => {
    const onBack = vi.fn();
    render(
      <I18nProvider>
        <MobileAppPageHeader onBack={onBack} title="连接器" />
      </I18nProvider>,
    );

    screen.getByRole("heading", { name: "连接器" });
    const back = screen.getByRole("button", { name: /back|返回/i });

    fireEvent.click(back);
    expect(onBack).toHaveBeenCalledOnce();
  });
});
