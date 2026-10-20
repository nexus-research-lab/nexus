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
    status: {
      installed: true,
      enabled: true,
      version: "0.1.0",
      permissions: {
        screen_capture: "granted",
        accessibility: "denied",
        input_control: "not_determined",
      },
    },
  });
});

it("renders CUA in its own settings page and keeps the switch action local", async () => {
  const user = userEvent.setup();
  renderSettings();
  const controller = useController.mock.results[0]?.value as {
    onChange: ReturnType<typeof vi.fn>;
    onOpenPermissions: ReturnType<typeof vi.fn>;
  };

  expect(screen.getByRole("heading", { name: "电脑操控" })).toBeTruthy();
  const toggle = screen.getByRole("switch", { name: "允许 Nexus 操作电脑" });
  expect(toggle.getAttribute("aria-checked")).toBe("true");
  expect(screen.getByText("仍有权限未开启，请按下面状态处理")).toBeTruthy();
  expect(screen.getByText("屏幕录制")).toBeTruthy();
  expect(screen.getByText("已开启")).toBeTruthy();
  expect(screen.getByText("辅助功能")).toBeTruthy();
  expect(screen.getByText("未开启")).toBeTruthy();
  expect(screen.getByText("输入控制")).toBeTruthy();
  expect(screen.getByText("待确认")).toBeTruthy();
  expect(screen.getAllByRole("button", { name: "开启" })).toHaveLength(2);
  await user.click(screen.getAllByRole("button", { name: "开启" })[0]);
  expect(controller.onOpenPermissions).toHaveBeenCalledExactlyOnceWith("accessibility");
  await user.click(toggle);
  expect(controller.onChange).toHaveBeenCalledExactlyOnceWith(false);
});

it("shows a complete permission result without a next-step action", () => {
  useController.mockReturnValue({
    busy: false,
    error: false,
    onChange: vi.fn(),
    onCheckPermissions: vi.fn(),
    onOpenPermissions: vi.fn(),
    status: {
      installed: true,
      enabled: true,
      version: "0.1.0",
      permissions: {
        screen_capture: "granted",
        accessibility: "granted",
        input_control: "granted",
      },
    },
  });

  renderSettings();

  expect(screen.getByText("权限已全部开启，可以操作电脑")).toBeTruthy();
  expect(screen.queryByRole("button", { name: /开启下一项/ })).toBeNull();
  expect(screen.getByRole("button", { name: "重新检查权限" })).toBeTruthy();
});
