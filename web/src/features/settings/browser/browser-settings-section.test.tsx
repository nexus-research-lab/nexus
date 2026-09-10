// INPUT: Browser 扩展已连接或状态读取失败，以及稳定的 Preferences 快照。
// OUTPUT: 证明 Browser 设置复用共享 Typography、Badge、ResourceState 与 Settings Shape。
// POS: Browser 视图合同测试；轮询协议与 Preferences 事务由各自模型/接口测试负责。

import { act, fireEvent, render, screen } from "@testing-library/react";
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

  it("binds each CDP switch to its own risk and description while retaining the permission gate", async () => {
    mocks.getStatus.mockResolvedValue({ connected: false, connection_state: "disconnected" });
    const { rerender } = renderWithI18n(<><BrowserSettingsSection /><BrowserSettingsSection /></>);
    expect(await screen.findAllByRole("button", { name: "settings.browser.install_action" })).toHaveLength(2);
    const controls = screen.getAllByRole("switch", { name: "settings.browser.cdp_toggle" });
    const refs = controls.map((control) => control.getAttribute("aria-describedby")!.split(" "));
    expect(new Set(refs.flat()).size).toBe(4);
    for (const ids of refs) {
      expect(ids.map((id) => document.getElementById(id)?.textContent))
        .toEqual(["settings.browser.cdp_risk", "settings.browser.cdp_description"]);
    }
    fireEvent.click(document.getElementById(refs[0][0])!);
    expect(mocks.updatePreferences).not.toHaveBeenCalled();
    fireEvent.click(controls[1]);
    expect(mocks.updatePreferences).toHaveBeenCalledOnce();
    const current = { browser_cdp_enabled: false, version: 7, emotion_enabled: true };
    expect(mocks.updatePreferences.mock.calls[0][0](current)).toEqual({ ...current, browser_cdp_enabled: true });
    mocks.preferences.writable = false;
    rerender(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <BrowserSettingsSection />
    </I18N_CONTEXT.Provider>);
    const control = screen.getByRole("switch", { name: "settings.browser.cdp_toggle" }) as HTMLButtonElement;
    expect(control.disabled).toBe(true);
    fireEvent.click(control);
    expect(mocks.updatePreferences).toHaveBeenCalledOnce();
    await screen.findByRole("button", { name: "settings.browser.install_action" });
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

  it("uses the shared compact Spinner while opening desktop setup", async () => {
    mocks.getStatus.mockResolvedValue({
      browser_name: "Chrome",
      connected: false,
      connection_state: "disconnected",
    });
    mocks.openSetup.mockReturnValue(new Promise(() => undefined));

    const { container } = renderWithI18n(<BrowserSettingsSection />);
    const setupButton = await screen.findByRole("button", {
      name: "settings.browser.install_action",
    });
    fireEvent.click(setupButton);

    const spinner = container.querySelector("svg.animate-spin");
    expect(spinner?.getAttribute("class")).toContain("h-3.5 w-3.5");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
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
