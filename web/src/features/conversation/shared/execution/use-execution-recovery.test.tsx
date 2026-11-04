// INPUT: Recovery API acknowledgements, lost responses and changing session identity.
// OUTPUT: Duplicate requests are fenced and unknown writes are reconciled by reads.
// POS: Recovery action state and scope lifecycle regression.
import { StrictMode, type ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import type { ExecutionGraphNodeView, ExecutionRecoveryReceipt } from "@/types/conversation/execution";
import { RECOVERY_REQUEUE_MS, useExecutionRecovery } from "./use-execution-recovery";

const api = vi.hoisted(() => ({ request: vi.fn(), read: vi.fn() }));
vi.mock("@/lib/api/conversation/execution-api", () => ({ requestExecutionRecoveryApi: api.request, getExecutionRecoveryReceiptApi: api.read }));
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
