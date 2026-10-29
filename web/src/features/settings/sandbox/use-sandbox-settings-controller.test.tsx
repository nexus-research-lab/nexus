// INPUT: Real sandbox controller with isolated API and preference storage.
// OUTPUT: Explicit diagnostic reads stay separate from preference mutations; the toggle uses one.
// POS: Sandbox settings behavior; visual rendering remains in the sandbox view tests.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useSandboxSettingsController } from "./use-sandbox-settings-controller";

const mocks = vi.hoisted(() => ({ read: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/settings/runtime-api", () => ({
  getNxsRuntimeStatusApi: mocks.read,
}));
vi.mock("../general/use-user-preferences", () => ({ useUserPreferences: () => ({ preferences: { desktop_sandbox_enabled: false }, loading: false, saving: false, writable: true, updatePreferences: mocks.update }) }));
beforeEach(() => vi.resetAllMocks());

it.each(["unsupported", "dependencies_available"])("diagnoses %s without changing preferences", async (state) => {
  mocks.read.mockResolvedValue({ available: true, sandbox: { state } });
  const { result } = renderHook(useSandboxSettingsController);
  await act(() => result.current.onCheckSandbox());
  expect(mocks.read).toHaveBeenCalledExactlyOnceWith(true);
  expect(result.current.sandboxState).toBe(state);
  expect(mocks.update).not.toHaveBeenCalled();
  expect(result.current.preferencesBusy).toBe(false);
});

it("retains unknown for old servers and failures", async () => {
  mocks.read.mockResolvedValueOnce({ available: true }).mockRejectedValueOnce(new Error("offline"));
  const { result } = renderHook(useSandboxSettingsController);
  await act(() => result.current.onCheckSandbox());
  expect(result.current.sandboxState).toBe("unknown");
  await act(() => result.current.onCheckSandbox());
  expect(result.current.sandboxState).toBe("unknown");
  expect(result.current.sandboxChecking).toBe(false);
  expect(mocks.update).not.toHaveBeenCalled();
});

it("does not launch duplicate checks or lock preference changes", async () => {
  let resolve!: (value: unknown) => void;
  mocks.read.mockReturnValue(new Promise((done) => { resolve = done; }));
  const { result } = renderHook(useSandboxSettingsController);
  let pending!: Promise<void>;
  act(() => { pending = result.current.onCheckSandbox(); void result.current.onCheckSandbox(); });
  expect(mocks.read).toHaveBeenCalledTimes(1);
  expect(result.current.sandboxChecking).toBe(true);
  expect(result.current.preferencesBusy).toBe(false);
  await act(async () => { resolve({ available: true, sandbox: { state: "unsupported" } }); await pending; });
  expect(mocks.update).not.toHaveBeenCalled();
});

it("toggles the desktop sandbox through one preference update", () => {
  const { result } = renderHook(useSandboxSettingsController);
  expect(result.current.desktopSandboxEnabled).toBe(false);
  act(() => result.current.onDesktopSandboxEnabledChange(true));
  expect(mocks.update).toHaveBeenCalledOnce();
  const apply = mocks.update.mock.calls[0][0] as (current: Record<string, unknown>) => Record<string, unknown>;
  expect(apply({ agent_runtime_kind: "nxs", desktop_sandbox_enabled: false })).toEqual({
    agent_runtime_kind: "nxs",
    desktop_sandbox_enabled: true,
  });
  expect(mocks.read).not.toHaveBeenCalled();
});
