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
  expect(screen.getByText("仅操作你授权的应用 · 组件 0.1.0")).toBeTruthy();
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
  expect(screen.getByText("2 项待处理")).toBeTruthy();
  expect(screen.getByText("屏幕录制")).toBeTruthy();
  expect(screen.getByText("已开启")).toBeTruthy();
  expect(screen.getByText("辅助功能")).toBeTruthy();
  expect(screen.getByText("未开启")).toBeTruthy();
  expect(screen.getByText("输入控制")).toBeTruthy();
  expect(screen.getByText("待确认")).toBeTruthy();
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(screen.getByRole("button", { name: "设置权限" }));
  expect(controller.onOpenPermissions).toHaveBeenCalledOnce();
  await user.click(toggle);
  expect(controller.onChange).toHaveBeenCalledExactlyOnceWith(false);
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
  // 已开启只保留给读屏，界面上由勾选图标表达。
  expect(screen.getAllByText("已开启").every((node) => node.classList.contains("sr-only"))).toBe(true);
  expect(screen.queryByRole("button", { name: "设置权限" })).toBeNull();
  expect(screen.getByRole("button", { name: "重新检查权限" })).toBeTruthy();
});
