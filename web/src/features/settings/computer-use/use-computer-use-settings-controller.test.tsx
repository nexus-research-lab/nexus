// INPUT: Real Computer Use controller with isolated API and desktop bridge.
// OUTPUT: Permission guidance reaches the dedicated bridge command; install precedes enable.
// POS: Regression for the dead external-URL permission button; rendering stays in the section test.
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useComputerUseSettingsController } from "./use-computer-use-settings-controller";

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  install: vi.fn(),
  setEnabled: vi.fn(),
  checkPermissions: vi.fn(),
  permissionSetup: vi.fn(),
  syncGuide: vi.fn(),
  openExternalURL: vi.fn(),
}));
vi.mock("@/lib/api/settings/runtime-api", () => ({
  getComputerUseStatusApi: mocks.status,
  installComputerUseApi: mocks.install,
  setComputerUseEnabledApi: mocks.setEnabled,
  checkComputerUsePermissionsApi: mocks.checkPermissions,
}));
vi.mock("@/lib/desktop-bridge/desktop-bridge", () => ({
  startDesktopComputerUsePermissionSetup: mocks.permissionSetup,
  updateDesktopComputerUsePermissionSetup: mocks.syncGuide,
  openDesktopExternalURL: mocks.openExternalURL,
}));
beforeEach(() => {
  vi.resetAllMocks();
  mocks.syncGuide.mockResolvedValue({ updated: true });
  window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop", platform: "macos" };
});
afterEach(() => { delete window.__NEXUS_DESKTOP_RUNTIME__; });

it.each([false, true])("updates a legacy installation while preserving enabled=%s", async (enabled) => {
  const legacy = { installed: true, enabled, version: "0.1.0", required_version: "0.2.0", update_required: true };
  const updated = { ...legacy, version: "0.2.0", update_required: false };
  mocks.status.mockResolvedValue(legacy);
  mocks.install.mockResolvedValue(updated);
  mocks.checkPermissions.mockResolvedValue(updated);
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status?.update_required).toBe(true));
  expect(mocks.checkPermissions).not.toHaveBeenCalled();
  await act(() => result.current.onUpdate());
  expect(mocks.install).toHaveBeenCalledOnce();
  expect(mocks.setEnabled).not.toHaveBeenCalled();
  expect(mocks.checkPermissions).toHaveBeenCalledTimes(enabled ? 1 : 0);
  expect(result.current.status).toEqual(updated);
});

it("installs the required version before enabling an already installed legacy component", async () => {
  mocks.status.mockResolvedValue({ installed: true, enabled: false, version: "0.1.0", update_required: true });
  mocks.install.mockResolvedValue({ installed: true, enabled: false, version: "0.2.0" });
  mocks.setEnabled.mockResolvedValue({ installed: true, enabled: true, version: "0.2.0" });
  mocks.checkPermissions.mockResolvedValue({ installed: true, enabled: true, version: "0.2.0" });
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status?.version).toBe("0.1.0"));
  await act(() => result.current.onChange(true));
  expect(mocks.install).toHaveBeenCalledOnce();
  expect(mocks.install.mock.invocationCallOrder[0]).toBeLessThan(mocks.setEnabled.mock.invocationCallOrder[0]);
  expect(result.current.status?.version).toBe("0.2.0");
});

it("keeps a failed upgrade visible and retryable without enabling the legacy component", async () => {
  const legacy = { installed: true, enabled: false, version: "0.1.0", update_required: true };
  mocks.status.mockResolvedValue(legacy);
  mocks.install.mockRejectedValue(new Error("download failed"));
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status).toEqual(legacy));
  await act(() => result.current.onUpdate());
  expect(result.current.status).toEqual(legacy);
  expect(result.current.error).toBe("component");
  expect(result.current.busy).toBe(false);
  expect(mocks.setEnabled).not.toHaveBeenCalled();
});

it("guides permissions through the dedicated bridge command, never the external URL allowlist", async () => {
  mocks.status.mockResolvedValue({ installed: true, enabled: true });
  mocks.checkPermissions.mockResolvedValue({ installed: true, enabled: true });
  mocks.permissionSetup.mockResolvedValue({ opened: true });
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status?.enabled).toBe(true));
  await act(() => result.current.onOpenPermissions());
  expect(mocks.permissionSetup).toHaveBeenCalledExactlyOnceWith("zh", undefined);
  expect(mocks.openExternalURL).not.toHaveBeenCalled();
  expect(result.current.error).toBeNull();
});

it.each(["macos", "windows"])("installs and probes once on %s, opening only the macOS wizard", async (platform) => {
  window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop", platform };
  mocks.status.mockResolvedValue({ installed: false, enabled: false });
  mocks.install.mockResolvedValue({ installed: true, enabled: false });
  mocks.setEnabled.mockResolvedValue({ installed: true, enabled: true });
  mocks.checkPermissions.mockResolvedValue({
    installed: true,
    enabled: true,
    permissions: { screen_capture: "not_determined", accessibility: "granted", input_control: "granted" },
  });
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status?.enabled).toBe(false));
  await act(() => result.current.onChange(true));
  expect(mocks.install).toHaveBeenCalledExactlyOnceWith();
  expect(mocks.setEnabled).toHaveBeenCalledExactlyOnceWith(true);
  expect(mocks.checkPermissions).toHaveBeenCalledOnce();
  expect(mocks.permissionSetup).toHaveBeenCalledTimes(platform === "macos" ? 1 : 0);
  expect(result.current.status?.permissions?.screen_capture).toBe("not_determined");
  expect(result.current.error).toBeNull();
});

it("rechecks after returning from the system permission page", async () => {
  mocks.status.mockResolvedValue({
    installed: true,
    enabled: true,
    permissions: { screen_capture: "denied", accessibility: "denied", input_control: "denied" },
  });
  mocks.checkPermissions
    .mockResolvedValueOnce({
      installed: true,
      enabled: true,
      permissions: { screen_capture: "denied", accessibility: "denied", input_control: "denied" },
    })
    .mockResolvedValueOnce({
      installed: true,
      enabled: true,
      permissions: { screen_capture: "granted", accessibility: "denied", input_control: "denied" },
    });
  mocks.permissionSetup.mockResolvedValue({ opened: true, permission: "screen_capture" });
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));

  await waitFor(() => expect(result.current.status?.permissions?.screen_capture).toBe("denied"));
  expect(mocks.permissionSetup).not.toHaveBeenCalled();
  await act(() => result.current.onOpenPermissions());
  await act(async () => {
    window.dispatchEvent(new Event("nexus:computer-use-check"));
  });

  await waitFor(() => expect(mocks.checkPermissions).toHaveBeenCalledTimes(2));
  expect(result.current.status?.permissions?.screen_capture).toBe("granted");
  expect(mocks.syncGuide).toHaveBeenCalledWith({ permissions: result.current.status?.permissions });
});

it("keeps the guide closed when enabling with every permission granted", async () => {
  mocks.status.mockResolvedValue({ installed: true, enabled: false });
  mocks.setEnabled.mockResolvedValue({ installed: true, enabled: true });
  mocks.checkPermissions.mockResolvedValue({ installed: true, enabled: true,
    permissions: { screen_capture: "granted", accessibility: "granted", input_control: "granted" },
  });
  const { result } = renderHook(() => useComputerUseSettingsController("zh"));
  await waitFor(() => expect(result.current.status?.enabled).toBe(false));
  await act(() => result.current.onChange(true));
  expect(mocks.permissionSetup).not.toHaveBeenCalled();
});
