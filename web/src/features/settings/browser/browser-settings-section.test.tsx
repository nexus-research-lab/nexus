// INPUT: Browser 扩展已连接或状态读取失败，以及稳定的 Preferences 快照。
// OUTPUT: 证明 Browser 设置复用共享 Typography、Badge、ResourceState 与 Settings Shape。
// POS: Browser 视图合同测试；轮询协议与 Preferences 事务由各自模型/接口测试负责。

import { act, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { BrowserSettingsSection } from "./browser-settings-section";

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  openSetup: vi.fn(),
  updatePreferences: vi.fn(),
  preferences: { loading: false, saving: false, writable: true },
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
    preferences: { browser_cdp_enabled: false },
    recovery: undefined,
    updatePreferences: mocks.updatePreferences,
    ...mocks.preferences,
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
    Object.assign(mocks.preferences, { loading: false, saving: false, writable: true });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("uses the shared recoverable error state when status cannot be read", async () => {
    mocks.getStatus.mockRejectedValue(new Error("offline"));

    const { container } = renderWithI18n(<BrowserSettingsSection />);

    expect(await screen.findByText("settings.browser.status_failed")).toBeTruthy();
    expect(container.querySelector('[data-resource-state="error"]')).toBeTruthy();
    expect(screen.getByRole("button", { name: "settings.browser.refresh" })).toBeTruthy();
  });

});

it("does not overlap slow extension status polls and stops after unmount", async () => {
  vi.useFakeTimers();
  mocks.getStatus.mockReset();
  let resolveStatus!: (value: object) => void;
  mocks.getStatus.mockImplementation(() => new Promise((resolve) => { resolveStatus = resolve; }));
  const { unmount } = renderWithI18n(<BrowserSettingsSection />);
  expect(mocks.getStatus).toHaveBeenCalledOnce();
  await act(async () => { await vi.advanceTimersByTimeAsync(6000); });
  expect(mocks.getStatus).toHaveBeenCalledOnce();
  await act(async () => { resolveStatus({ connected: false, connection_state: "disconnected" }); });
  await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
  expect(mocks.getStatus).toHaveBeenCalledTimes(2);
  unmount();
  await act(async () => { resolveStatus({ connected: true, browser_name: "Late" }); await vi.advanceTimersByTimeAsync(6000); });
  expect(mocks.getStatus).toHaveBeenCalledTimes(2);
  vi.useRealTimers();
});
