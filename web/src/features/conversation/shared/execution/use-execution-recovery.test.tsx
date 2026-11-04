// INPUT: Recovery API acknowledgements, lost responses and changing session identity.
// OUTPUT: Duplicate requests are fenced and unknown writes are reconciled by reads.
// POS: Recovery action state and scope lifecycle regression.
import { StrictMode, type ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import type { ExecutionGraphNodeView, ExecutionRecoveryReceipt } from "@/types/conversation/execution";
import { RECOVERY_REQUEUE_MS, useExecutionRecovery } from "./use-execution-recovery";

const api = vi.hoisted(() => ({ request: vi.fn(), read: vi.fn(), preview: vi.fn(), retry: vi.fn() }));
vi.mock("@/lib/api/conversation/execution-api", () => ({ requestExecutionRecoveryApi: api.request, getExecutionRecoveryReceiptApi: api.read, getWorkRetryPreviewApi: api.preview, requestWorkRetryApi: api.retry }));
const node = { id: "node", work_item_id: "work", attempt_id: "attempt", can_recover: true } as ExecutionGraphNodeView;
const wrapper = ({ children }: { children: ReactNode }) => <StrictMode>{children}</StrictMode>;

it("fences same-frame clicks and uses reads after a lost acknowledgement", async () => {
  api.request.mockReset(); api.read.mockReset();
  let reject!: (reason: Error) => void;
  api.request.mockReturnValue(new Promise<ExecutionRecoveryReceipt>((_, fail) => { reject = fail; }));
  const refresh = vi.fn();
  const { result } = renderHook(() => useExecutionRecovery("session", "execution", refresh), { wrapper });
  act(() => { result.current.run(node); result.current.run(node); });
  expect(api.request).toHaveBeenCalledOnce();
  const requestId = api.request.mock.calls[0][4] as string;
  await act(async () => reject(new Error("connection lost")));
  expect(result.current.states.attempt.status).toBe("unknown");
  api.read.mockResolvedValue({ execution_id: "execution", work_item_id: "work", attempt_id: "attempt", request_id: requestId, queued: true });
  await act(async () => result.current.run(node));
  expect(api.read).toHaveBeenCalledWith("session", "execution", requestId);
  expect(api.request).toHaveBeenCalledOnce();
  expect(result.current.states.attempt.status).toBe("queued");
  expect(refresh).toHaveBeenCalledOnce();
});

it("rejects stale scope completions and never requests an unauthorized node", async () => {
  api.request.mockReset();
  let resolve!: (value: ExecutionRecoveryReceipt) => void;
  api.request.mockReturnValue(new Promise<ExecutionRecoveryReceipt>((finish) => { resolve = finish; }));
  const refresh = vi.fn();
  const { result, rerender } = renderHook(({ session }) => useExecutionRecovery(session, "execution", refresh), { initialProps: { session: "first" } });
  act(() => { void result.current.run({ ...node, can_recover: false }); });
  expect(api.request).not.toHaveBeenCalled();
  act(() => { void result.current.run(node); });
  const requestId = api.request.mock.calls[0][4] as string;
  rerender({ session: "second" }); rerender({ session: "first" });
  await act(async () => resolve({ execution_id: "execution", work_item_id: "work", attempt_id: "attempt", request_id: requestId, queued: true }));
  expect(result.current.states).toEqual({});
  expect(refresh).not.toHaveBeenCalled();
});

it("releases a queued fence after the cooldown so the same Attempt can be retried", async () => {
  vi.useFakeTimers();
  try {
    api.request.mockReset();
    api.request.mockImplementation(async (_s, execution, work, attempt, requestId) => ({ execution_id: execution, work_item_id: work, attempt_id: attempt, request_id: requestId, queued: true }));
    const { result } = renderHook(() => useExecutionRecovery("session", "execution", vi.fn()));
    await act(async () => result.current.run(node));
    expect(result.current.states.attempt.status).toBe("queued");
    await act(async () => result.current.run(node));
    expect(api.request).toHaveBeenCalledOnce();
    await act(async () => { vi.advanceTimersByTime(RECOVERY_REQUEUE_MS); });
    expect(result.current.states.attempt).toBeUndefined();
    await act(async () => result.current.run(node));
    expect(api.request).toHaveBeenCalledTimes(2);
    expect(api.request.mock.calls[1][4]).not.toBe(api.request.mock.calls[0][4]);
  } finally {
    vi.useRealTimers();
  }
});


it("previews submitted dependencies, confirms the exact revision, and reconciles a lost retry response", async () => {
  api.preview.mockReset(); api.retry.mockReset(); api.read.mockReset();
  const preview = { execution_id: "execution", work_item_id: "work", attempt_id: "attempt", snapshot_revision: 7, submitted: true, affected: [{ work_item_id: "work", subject: "Draft", running: false, submitted: true }, { work_item_id: "child", subject: "Publish", running: true, submitted: false }] };
  api.preview.mockResolvedValue(preview);
  const { result } = renderHook(() => useExecutionRecovery("session", "execution", vi.fn()));
  const retryNode = { ...node, can_retry: true };
  await act(async () => result.current.run(retryNode));
  expect(result.current.confirmation).toEqual(preview);
  expect(api.retry).not.toHaveBeenCalled();
  act(() => result.current.cancelRetry?.());
  expect(result.current.confirmation).toBeNull();
  await act(async () => result.current.run(retryNode));
  api.retry.mockRejectedValue(new Error("lost response"));
  await act(async () => result.current.confirmRetry?.());
  expect(api.retry).toHaveBeenCalledWith("session", preview, expect.any(String));
  expect(result.current.states.attempt.status).toBe("unknown");
  const requestId = api.retry.mock.calls[0][2];
  api.read.mockResolvedValue({ execution_id: "execution", work_item_id: "work", attempt_id: "attempt", request_id: requestId, queued: true });
  await act(async () => result.current.run(retryNode));
  expect(api.retry).toHaveBeenCalledOnce();
  expect(api.read).toHaveBeenCalledWith("session", "execution", requestId);
});

it("retries an unsubmitted task from a fresh preview without downstream confirmation", async () => {
  api.preview.mockReset(); api.retry.mockReset();
  const preview = { execution_id: "execution", work_item_id: "work", attempt_id: "attempt", snapshot_revision: 9, submitted: false, affected: [] };
  api.preview.mockResolvedValue(preview);
  api.retry.mockImplementation(async (_session, value, requestId) => ({ ...value, request_id: requestId, queued: true }));
  const { result } = renderHook(() => useExecutionRecovery("session", "execution", vi.fn()));
  await act(async () => result.current.run({ ...node, can_retry: true }));
  expect(result.current.confirmation).toBeNull();
  expect(api.retry).toHaveBeenCalledWith("session", preview, expect.any(String));
});
