// INPUT: Selected history scope and deferred read responses.
// OUTPUT: Exact lazy reads, stale retention, and access/selection fences.
// POS: History detail resource regression tests.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { getExecutionApi } from "@/lib/api/conversation/execution-api";
import { UnauthorizedError } from "@/lib/api/core/http-error";
import type { ExecutionView } from "@/types/conversation/execution";
import { useExecutionViewResource } from "./use-execution-view-resource";
vi.mock("@/lib/api/conversation/execution-api", () => ({ getExecutionApi: vi.fn() }));
const view = { id: "selected-a", session_key: "a" } as ExecutionView;
beforeEach(() => vi.resetAllMocks());
it("reads only selected details and fences closed or switched selections", async () => {
  let finish!: (value: ExecutionView) => void;
  vi.mocked(getExecutionApi).mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockRejectedValue(new Error("selection unavailable"));
  const hook = renderHook(({ id }) => useExecutionViewResource("a", id), { initialProps: { id: null as string | null } });
  expect(getExecutionApi).not.toHaveBeenCalled();
  hook.rerender({ id: "selected-a" });
  expect(getExecutionApi).toHaveBeenCalledExactlyOnceWith("a", "selected-a");
  hook.rerender({ id: "selected-b" });
  await act(async () => finish(view));
  expect(hook.result.current.execution).toBeNull();
  hook.rerender({ id: null });
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
  expect(getExecutionApi).toHaveBeenCalledTimes(2);
});
it("retains same-selection stale data, but access loss and session changes discard it", async () => {
  vi.mocked(getExecutionApi).mockResolvedValue(view);
  const hook = renderHook(({ session }) => useExecutionViewResource(session, "selected-a"), { initialProps: { session: "a" } });
  await waitFor(() => expect(hook.result.current.execution).toEqual(view));
  vi.mocked(getExecutionApi).mockRejectedValue(new Error("offline"));
  act(() => hook.result.current.refresh());
  await waitFor(() => expect(hook.result.current.isStale).toBe(true));
  vi.mocked(getExecutionApi).mockRejectedValue(new UnauthorizedError("expired"));
  act(() => hook.result.current.refresh());
  await waitFor(() => expect(hook.result.current.execution).toBeNull());
  vi.mocked(getExecutionApi).mockRejectedValue(new Error("still offline"));
  act(() => hook.result.current.refresh());
  await waitFor(() => expect(hook.result.current.error).toBeTruthy());
  expect(hook.result.current.execution).toBeNull();
  hook.rerender({ session: "b" });
  expect(hook.result.current.execution).toBeNull();
});
