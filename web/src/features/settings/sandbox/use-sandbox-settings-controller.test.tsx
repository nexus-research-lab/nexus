// INPUT: Real sandbox controller with isolated API and preference storage.
// OUTPUT: Explicit diagnostic reads stay separate from preference mutations; the toggle uses one.
// POS: Sandbox settings behavior; visual rendering remains in the sandbox view tests.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useSandboxSettingsController } from "./use-sandbox-settings-controller";

const mocks = vi.hoisted(() => ({ discard: vi.fn(), inspect: vi.fn(), read: vi.fn(), reconcile: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/settings/runtime-api", () => ({
  discardSandboxRecoveryApi: mocks.discard,
  getNxsRuntimeStatusApi: mocks.read,
  inspectSandboxResourcesApi: mocks.inspect,
  reconcileSandboxResourcesApi: mocks.reconcile,
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
  const { result } = renderHook(useSandboxSettingsController);

  await act(() => result.current.onInspectSandboxResources());
  expect(mocks.inspect).toHaveBeenCalledOnce();
  expect(mocks.reconcile).toHaveBeenNthCalledWith(1, { older_than_seconds: 3600, apply: false });
  expect(result.current.sandboxRecoverySummary).toEqual({
    blockedCount: 0,
    candidateCount: 1,
    discardableLaunchIds: [],
    discardedCount: 0,
    removedCount: 0,
    resourceCount: 2,
    unknownCount: 1,
  });

  await act(() => result.current.onReconcileSandboxResources());
  expect(mocks.reconcile).toHaveBeenNthCalledWith(2, { older_than_seconds: 3600, apply: true });
  expect(mocks.reconcile).toHaveBeenNthCalledWith(3, { older_than_seconds: 3600, apply: false });
  expect(mocks.inspect).toHaveBeenCalledTimes(2);
  expect(result.current.sandboxRecoverySummary).toEqual({
    blockedCount: 0,
    candidateCount: 0,
    discardableLaunchIds: [],
    discardedCount: 0,
    removedCount: 1,
    resourceCount: 1,
    unknownCount: 1,
  });
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

it("discards only the records the server marked discardable, then re-inspects", async () => {
  const preview = { owner_user_id: "owner", older_than_seconds: 3600, apply: false, candidates: [], removed: [], skipped: [] };
  mocks.inspect
    .mockResolvedValueOnce({
      owner_user_id: "owner",
      resources: [],
      blocked_sessions: [
        { launch_id: "a", session_key: "dm", stage: "process", discardable: false },
        { launch_id: "b", session_key: "room", stage: "lifecycle", discardable: true },
      ],
    })
    .mockResolvedValueOnce({
      owner_user_id: "owner",
      resources: [],
      blocked_sessions: [{ launch_id: "a", session_key: "dm", stage: "process", discardable: false }],
    });
  mocks.reconcile.mockResolvedValue(preview);
  mocks.discard.mockResolvedValue({ launch_id: "b", discarded: true });
  const { result } = renderHook(useSandboxSettingsController);
  await act(() => result.current.onInspectSandboxResources());
  expect(result.current.sandboxRecoverySummary?.blockedCount).toBe(2);
  expect(result.current.sandboxRecoverySummary?.discardableLaunchIds).toEqual(["b"]);

  await act(() => result.current.onDiscardBlockedSandboxRecords());
  expect(mocks.discard).toHaveBeenCalledExactlyOnceWith("b");
  expect(mocks.inspect).toHaveBeenCalledTimes(2);
  expect(result.current.sandboxRecoverySummary).toMatchObject({ blockedCount: 1, discardableLaunchIds: [], discardedCount: 1 });
  expect(result.current.sandboxRecoveryError).toBe(false);
  expect(mocks.update).not.toHaveBeenCalled();
});
