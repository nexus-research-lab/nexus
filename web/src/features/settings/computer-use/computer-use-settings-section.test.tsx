import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { zhSettingsMessages } from "@/shared/i18n/catalog/zh/settings";

import { ComputerUseSettingsSection } from "./computer-use-settings-section";

const useController = vi.hoisted(() => vi.fn());
vi.mock(import("./use-computer-use-settings-controller"), async (original) => ({
  ...await original(),
  useComputerUseSettingsController: useController,
}));

const messages: Record<string, string> = zhSettingsMessages;

afterEach(() => {
  delete window.__NEXUS_DESKTOP_RUNTIME__;
});

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

it("offers the required component update for an installed legacy version", async () => {
  const onUpdate = vi.fn();
  useController.mockReturnValue({
    busy: false, error: false, onChange: vi.fn(), onUpdate,
    status: { installed: true, enabled: true, version: "0.1.0", required_version: "0.2.0", update_required: true },
  });
  renderSettings();
  expect(screen.getByText("0.1.0")).toBeTruthy();
  expect(screen.getByText("0.2.0")).toBeTruthy();
  expect(screen.getByText("当前 Nexus 需要电脑操控组件 0.2.0。")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "重新检查权限" })).toBeNull();
  await userEvent.setup().click(screen.getByRole("button", { name: "更新组件" }));
  expect(onUpdate).toHaveBeenCalledOnce();
});

beforeEach(() => {
  vi.clearAllMocks();
  window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop", platform: "macos" };
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

it("shows Windows permission status without macOS setup actions", () => {
  window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop", platform: "windows" };
  useController.mockReturnValue({
    busy: false,
    error: false,
    onChange: vi.fn(),
    onCheckPermissions: vi.fn(),
    onOpenPermissions: vi.fn(),
    status: {
      installed: true,
      enabled: true,
      permissions: {
        screen_capture: "not_applicable",
        accessibility: "not_applicable",
        input_control: "not_applicable",
      },
    },
  });
  renderSettings();
  expect(screen.getByText("Windows 无需这些全局权限开关，操作按应用授权执行")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "设置权限" })).toBeNull();
  expect(screen.queryByText("不适用")).toBeNull();
});

it("offers a retry action matching the failed step", async () => {
  const onCheckPermissions = vi.fn();
  useController.mockReturnValue({
    busy: false,
    error: "check",
    onChange: vi.fn(),
    onCheckPermissions,
    onOpenPermissions: vi.fn(),
    status: { installed: true, enabled: true, version: "0.2.0", permissions: {} },
  });
  renderSettings();
  const alert = screen.getByRole("alert");
  expect(alert.textContent).toContain("暂时无法读取电脑操控权限，请重新检查。");
  await userEvent.setup().click(within(alert).getByRole("button", { name: "重新检查权限" }));
  expect(onCheckPermissions).toHaveBeenCalledOnce();
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

  expect(screen.getByText("已全部开启")).toBeTruthy();
  expect(within(screen.getByRole("list", { name: "系统权限" })).getAllByText("已开启")).toHaveLength(3);
  expect(screen.queryByRole("button", { name: "设置权限" })).toBeNull();
  expect(screen.getByRole("button", { name: "重新检查权限" })).toBeTruthy();
});
