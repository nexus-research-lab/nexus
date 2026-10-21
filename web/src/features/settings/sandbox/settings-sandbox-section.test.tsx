// INPUT: Real sandbox settings view with an isolated controller.
// OUTPUT: Toggle, explicit diagnosis and preview-before-cleanup interactions.
// POS: Sandbox view regression; API and preference transactions stay in controller tests.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";

import { SettingsSandboxSection } from "./settings-sandbox-section";

const useController = vi.hoisted(() => vi.fn());
vi.mock("./use-sandbox-settings-controller", () => ({ useSandboxSettingsController: useController }));

const messages: Record<string, string> = zhSettingsMessages;
const text = (key: string, params: Record<string, string | number> = {}) =>
  Object.entries(params).reduce(
    (value, [name, replacement]) => value.replace(`{${name}}`, String(replacement)),
    messages[`settings.runtime.${key}`],
  );

function configure() {
  const controller = {
    desktopSandboxEnabled: false,
    onDesktopSandboxEnabledChange: vi.fn(),
    loading: false,
    preferencesBusy: false,
    preferencesFeedback: null,
    preferencesRecovery: undefined,
    sandboxChecking: false,
    sandboxState: null,
    onCheckSandbox: vi.fn(),
    sandboxRecoveryApplying: false,
    sandboxRecoveryChecking: false,
    sandboxRecoveryError: false,
    sandboxRecoverySummary: null,
    onInspectSandboxResources: vi.fn(),
    onReconcileSandboxResources: vi.fn(),
  };
  useController.mockReturnValue(controller);
  return controller;
}

function Providers({ children }: { children: ReactNode }) {
  return (
    <MemoryRouter>
      <I18N_CONTEXT.Provider value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key, params) => Object.entries(params ?? {}).reduce(
          (value, [name, replacement]) => value.replace(`{${name}}`, String(replacement)),
          messages[key] ?? key,
        ),
      }}>
        {children}
      </I18N_CONTEXT.Provider>
    </MemoryRouter>
  );
}

function renderSettings() {
  return render(<SettingsSandboxSection />, { wrapper: Providers });
}

beforeEach(() => vi.clearAllMocks());

describe("desktop sandbox toggle", () => {
  it("switches the preference without running a diagnosis", async () => {
    const controller = configure();
    renderSettings();
    const toggle = screen.getByRole("switch", { name: messages["settings.general.desktop_sandbox_title"] });
    await userEvent.setup().click(toggle);
    expect(controller.onDesktopSandboxEnabledChange).toHaveBeenCalledExactlyOnceWith(true);
    expect(controller.onCheckSandbox).not.toHaveBeenCalled();
  });

  it("locks the toggle while preferences are saving", () => {
    const controller = configure();
    useController.mockReturnValue({ ...controller, preferencesBusy: true });
    renderSettings();
    expect(screen.getByRole("switch", { name: messages["settings.general.desktop_sandbox_title"] }).hasAttribute("disabled")).toBe(true);
  });
});

describe("sandbox diagnosis", () => {
  it("keeps diagnosis as an action rather than a second switch", () => {
    configure();
    renderSettings();
    expect(screen.getByRole("button", { name: text("sandbox_check") })).toBeTruthy();
    expect(screen.queryByRole("switch", { name: text("sandbox_check") })).toBeNull();
  });

  it("checks explicitly without changing the sandbox preference", async () => {
    const user = userEvent.setup();
    const controller = configure();
    renderSettings();
    await user.click(screen.getByRole("button", { name: text("sandbox_check") }));
    expect(controller.onCheckSandbox).toHaveBeenCalledOnce();
    expect(controller.onDesktopSandboxEnabledChange).not.toHaveBeenCalled();
  });

  it.each(["unknown", "unsupported", "missing_dependencies", "dependencies_available"])("shows %s", (state) => {
    const controller = configure();
    useController.mockReturnValue({ ...controller, sandboxState: state });
    renderSettings();
    expect(screen.getByRole("status").textContent).toBe(text(`sandbox_${state}`));
    expect(screen.getByRole("button", { name: text("sandbox_check") }).hasAttribute("disabled")).toBe(false);
  });
});

describe("sandbox recovery", () => {
  it("shows counts and keeps cleanup behind a separate explicit action", async () => {
    const controller = configure();
    useController.mockReturnValue({
      ...controller,
      sandboxRecoverySummary: {
        candidateCount: 2,
        removedCount: 0,
        resourceCount: 3,
        unknownCount: 1,
      },
    });
    renderSettings();
    const summary = screen.getByRole("status");
    expect(summary.textContent).toContain(text("sandbox_recovery_resources", { count: 3 }));
    expect(summary.textContent).toContain(text("sandbox_recovery_unknown", { count: 1 }));
    expect(screen.getByRole("button", { name: text("sandbox_recovery_apply") })).toBeTruthy();
    expect(controller.onInspectSandboxResources).not.toHaveBeenCalled();
    expect(controller.onReconcileSandboxResources).not.toHaveBeenCalled();
    await userEvent.setup().click(screen.getByRole("button", { name: text("sandbox_recovery_apply") }));
    expect(controller.onReconcileSandboxResources).toHaveBeenCalledOnce();
  });
});
