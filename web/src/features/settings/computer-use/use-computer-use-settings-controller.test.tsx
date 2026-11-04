// INPUT: Real Computer Use controller with isolated API and desktop bridge.
// OUTPUT: Permission guidance reaches the dedicated bridge command; install precedes enable.
// POS: Regression for the dead external-URL permission button; rendering stays in the section test.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useComputerUseSettingsController } from "./use-computer-use-settings-controller";

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  install: vi.fn(),
  setEnabled: vi.fn(),
  checkPermissions: vi.fn(),
  permissionSetup: vi.fn(),
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
  openDesktopExternalURL: mocks.openExternalURL,
}));
beforeEach(() => vi.resetAllMocks());

it("guides permissions through the dedicated bridge command, never the external URL allowlist", async () => {
  mocks.status.mockResolvedValue({ installed: true, enabled: true });
  mocks.checkPermissions.mockResolvedValue({ installed: true, enabled: true });
  mocks.permissionSetup.mockResolvedValue({ opened: true });
  const { result } = renderHook(useComputerUseSettingsController);
  await waitFor(() => expect(mocks.status).toHaveBeenCalledOnce());
  await act(() => result.current.onOpenPermissions("screen_capture"));
  expect(mocks.permissionSetup).toHaveBeenCalledExactlyOnceWith("screen_capture");
  expect(mocks.openExternalURL).not.toHaveBeenCalled();
  expect(result.current.error).toBe(false);
});

it("installs before enabling and then reads the permission state once", async () => {
  mocks.status.mockResolvedValue({ installed: false, enabled: false });
  mocks.install.mockResolvedValue({ installed: true, enabled: false });
  mocks.setEnabled.mockResolvedValue({ installed: true, enabled: true });
  mocks.checkPermissions.mockResolvedValue({
    installed: true,
    enabled: true,
    permissions: { screen_capture: "not_determined", accessibility: "granted", input_control: "granted" },
  });
  const { result } = renderHook(useComputerUseSettingsController);
  await act(() => result.current.onChange(true));
  expect(mocks.install).toHaveBeenCalledExactlyOnceWith();
  expect(mocks.setEnabled).toHaveBeenCalledExactlyOnceWith(true);
  expect(mocks.checkPermissions).toHaveBeenCalledOnce();
  expect(mocks.permissionSetup).not.toHaveBeenCalled();
  expect(result.current.status?.permissions?.screen_capture).toBe("not_determined");
  expect(result.current.error).toBe(false);
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
  const { result } = renderHook(useComputerUseSettingsController);

  await waitFor(() => expect(mocks.checkPermissions).toHaveBeenCalledOnce());
  await act(() => result.current.onOpenPermissions("screen_capture"));
  await act(async () => {
    window.dispatchEvent(new Event("focus"));
  });

  await waitFor(() => expect(mocks.checkPermissions).toHaveBeenCalledTimes(2));
  expect(result.current.status?.permissions?.screen_capture).toBe("granted");
});
