import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";

import { ComputerUseSettingsSection } from "./computer-use-settings-section";

const useController = vi.hoisted(() => vi.fn());
vi.mock("./use-computer-use-settings-controller", () => ({
  useComputerUseSettingsController: useController,
}));

const messages: Record<string, string> = zhSettingsMessages;

function renderSettings() {
  return render(
    <I18N_CONTEXT.Provider value={{
      locale: "zh",
      setLocale: vi.fn(),
      t: (key, params) => Object.entries(params ?? {}).reduce(
        (value, [name, replacement]) => value.replace(`{${name}}`, String(replacement)),
        messages[key] ?? key,
      ),
    }}>
      <ComputerUseSettingsSection />
    </I18N_CONTEXT.Provider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useController.mockReturnValue({
    busy: false,
    error: false,
    onChange: vi.fn(),
    onCheckPermissions: vi.fn(),
    onOpenPermissions: vi.fn(),
    status: { installed: true, enabled: false, version: "0.1.0" },
  });
});

it("renders CUA in its own settings page and keeps the switch action local", async () => {
  const user = userEvent.setup();
  renderSettings();
  const controller = useController.mock.results[0]?.value as { onChange: ReturnType<typeof vi.fn> };

  expect(screen.getByRole("heading", { name: "电脑操控" })).toBeTruthy();
  const toggle = screen.getByRole("switch", { name: "允许 Nexus 操作电脑" });
  expect(toggle.getAttribute("aria-checked")).toBe("false");
  await user.click(toggle);
  expect(controller.onChange).toHaveBeenCalledExactlyOnceWith(true);
});
