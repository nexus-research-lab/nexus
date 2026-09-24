// INPUT: Real runtime controller with isolated API and preference storage.
// OUTPUT: Explicit diagnostic reads stay separate from all preference mutations.
// POS: Sandbox check behavior; visual rendering remains in the settings view tests.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useRuntimeSettingsController } from "./use-runtime-settings-controller";

const mocks = vi.hoisted(() => ({ inspect: vi.fn(), read: vi.fn(), reconcile: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/settings/runtime-api", () => ({
  getNxsRuntimeStatusApi: mocks.read,
  inspectSandboxResourcesApi: mocks.inspect,
  reconcileSandboxResourcesApi: mocks.reconcile,
}));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("../general/use-user-preferences", () => ({ useUserPreferences: () => ({ preferences: { agent_runtime_kind: "nxs" }, loading: false, saving: false, writable: true, updatePreferences: mocks.update }) }));
beforeEach(() => vi.resetAllMocks());

it.each(["unsupported", "dependencies_available"])("diagnoses %s without changing preferences", async (state) => {
  mocks.read.mockResolvedValue({ available: true, sandbox: { state } });
  const { result } = renderHook(useRuntimeSettingsController);
  await act(() => result.current.onCheckSandbox());
  expect(mocks.read).toHaveBeenCalledExactlyOnceWith(true);
  expect(result.current.sandboxState).toBe(state);
  expect(mocks.update).not.toHaveBeenCalled();
  expect(result.current.preferencesBusy).toBe(false);
});

it("retains unknown for old servers and failures", async () => {
  mocks.read.mockResolvedValueOnce({ available: true }).mockRejectedValueOnce(new Error("offline"));
  const { result } = renderHook(useRuntimeSettingsController);
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
  const { result } = renderHook(useRuntimeSettingsController);
  let pending!: Promise<void>;
  act(() => { pending = result.current.onCheckSandbox(); void result.current.onCheckSandbox(); });
  expect(mocks.read).toHaveBeenCalledTimes(1);
  expect(result.current.sandboxChecking).toBe(true);
  expect(result.current.preferencesBusy).toBe(false);
  await act(async () => { resolve({ available: true, sandbox: { state: "unsupported" } }); await pending; });
  expect(mocks.update).not.toHaveBeenCalled();
});

it("previews owner resources before exposing an explicit cleanup action", async () => {
  const resource = { marker: { cleanup_state: "active" }, process_active: false };
  const unknown = { marker: { cleanup_state: "cleanup_unknown" }, process_active: true };
  mocks.inspect
    .mockResolvedValueOnce({ owner_user_id: "owner", resources: [resource, unknown] })
    .mockResolvedValueOnce({ owner_user_id: "owner", resources: [unknown] });
  mocks.reconcile
    .mockResolvedValueOnce({
      owner_user_id: "owner",
      older_than_seconds: 3600,
      apply: false,
      candidates: [resource],
      removed: [],
      skipped: [unknown],
    })
    .mockResolvedValueOnce({
      owner_user_id: "owner",
      older_than_seconds: 3600,
      apply: true,
      candidates: [resource],
      removed: [resource],
      skipped: [unknown],
    })
    .mockResolvedValueOnce({
      owner_user_id: "owner",
      older_than_seconds: 3600,
      apply: false,
      candidates: [],
      removed: [],
      skipped: [unknown],
    });
  const { result } = renderHook(useRuntimeSettingsController);

  await act(() => result.current.onInspectSandboxResources());
  expect(mocks.inspect).toHaveBeenCalledOnce();
  expect(mocks.reconcile).toHaveBeenNthCalledWith(1, { older_than_seconds: 3600, apply: false });
  expect(result.current.sandboxRecoverySummary).toEqual({
    candidateCount: 1,
    removedCount: 0,
    resourceCount: 2,
    unknownCount: 1,
  });

  await act(() => result.current.onReconcileSandboxResources());
  expect(mocks.reconcile).toHaveBeenNthCalledWith(2, { older_than_seconds: 3600, apply: true });
  expect(mocks.reconcile).toHaveBeenNthCalledWith(3, { older_than_seconds: 3600, apply: false });
  expect(mocks.inspect).toHaveBeenCalledTimes(2);
  expect(result.current.sandboxRecoverySummary).toEqual({
    candidateCount: 0,
    removedCount: 1,
    resourceCount: 1,
    unknownCount: 1,
  });
});
