// INPUT: Conversation 可靠性快照、只读资源状态和显式刷新动作。
// OUTPUT: 证明两类业务提示复用 UiInlineNotice，且失败映射、数据标记与动作条件不变。
// POS: Conversation reliability 视图合同测试；可靠性状态机本身归 hooks/agent/reliability。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ProviderUnavailableBanner } from "./provider-unavailable-banner";

vi.mock("@/features/onboarding/provider-setup/provider-setup-dialog", () => ({
  ProviderSetupDialog: ({ isOpen }: { isOpen: boolean }) => (
    isOpen ? <div aria-label="Provider setup" role="dialog" /> : null
  ),
}));

const I18N_VALUE = {
  locale: "zh" as const,
  setLocale: vi.fn(),
  t: (key: string) => key,
};

describe("Conversation reliability notices", () => {

  it("uses the same inline notice for Provider recovery and opens setup on demand", () => {
    render(
      <I18N_CONTEXT.Provider value={I18N_VALUE}>
        <ProviderUnavailableBanner compact />
      </I18N_CONTEXT.Provider>,
    );

    const status = screen.getByRole("status");
    expect(status.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(screen.getByText("onboarding.provider_setup_banner")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", {
      name: "onboarding.provider_setup_action",
    }));
    expect(screen.getByRole("dialog", { name: "Provider setup" })).toBeTruthy();
  });
});
