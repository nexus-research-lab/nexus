// INPUT: Browser 扩展已连接或状态读取失败，以及稳定的 Preferences 快照。
// OUTPUT: 证明 Browser 设置复用共享 Typography、Badge、ResourceState 与 Settings Shape。
// POS: Browser 视图合同测试；轮询协议与 Preferences 事务由各自模型/接口测试负责。

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { BrowserSettingsSection } from "./browser-settings-section";

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  openSetup: vi.fn(),
  updatePreferences: vi.fn(),
}));

vi.mock("@/lib/api/settings/browser-api", () => ({
  getBrowserExtensionStatusApi: mocks.getStatus,
}));

vi.mock("@/lib/desktop-bridge/desktop-bridge", () => ({
  startDesktopBrowserExtensionSetup: mocks.openSetup,
}));

vi.mock("../general/use-user-preferences", () => ({
  useUserPreferences: () => ({
    feedback: null,
    loading: false,
    preferences: { browser_cdp_enabled: false },
    recovery: undefined,
    saving: false,
    updatePreferences: mocks.updatePreferences,
    writable: true,
  }),
}));

function renderWithI18n(children: ReactNode) {
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

describe("Browser settings surface", () => {
  beforeEach(() => {
    mocks.getStatus.mockReset();
    mocks.openSetup.mockReset();
    mocks.updatePreferences.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("projects a connected extension through shared semantic owners", async () => {
    mocks.getStatus.mockResolvedValue({
      browser_name: "Chrome",
      connected: true,
      connection_state: "connected",
      extension_version: "1.2.3",
      protocol_version: "1",
    });

    const { container } = renderWithI18n(<BrowserSettingsSection />);

    const browserTitle = await screen.findByRole("heading", { name: "Chrome" });
    expect(browserTitle.className).toContain("ui-type-section-title");
    expect(screen.getByText("settings.browser.status_connected").className).toContain("var(--success)");
    expect(screen.getByText("settings.browser.status_version").className).toContain("ui-type-metadata");
    expect(screen.getByRole("heading", { name: "settings.browser.developer_title" }).className).toContain("ui-type-section-title");
    expect(container.querySelectorAll(".surface-radius-md").length).toBeGreaterThanOrEqual(2);
  });

  it("uses the shared recoverable error state when status cannot be read", async () => {
    mocks.getStatus.mockRejectedValue(new Error("offline"));

    const { container } = renderWithI18n(<BrowserSettingsSection />);

    expect(await screen.findByText("settings.browser.status_failed")).toBeTruthy();
    expect(container.querySelector('[data-resource-state="error"]')).toBeTruthy();
    expect(screen.getByRole("button", { name: "settings.browser.refresh" })).toBeTruthy();
  });
});
