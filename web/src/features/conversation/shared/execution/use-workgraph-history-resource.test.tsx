// INPUT: Real history hook, session changes and deferred API results.
// OUTPUT: History never crosses sessions; read failures retain only same-session snapshots.
// POS: WorkGraph history behavior regression, replacing source-string scope checks.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { getExecutionHistoryApi } from "@/lib/api/conversation/execution-api";
import { UnauthorizedError } from "@/lib/api/core/http-error";
import type { ExecutionSummary } from "@/types/conversation/execution";
import { useWorkGraphHistoryResource } from "./use-workgraph-history-resource";
vi.mock("@/lib/api/conversation/execution-api", () => ({ getExecutionHistoryApi: vi.fn() }));
const snapshot: ExecutionSummary[] = [{
  id: "history-a", session_key: "a", objective: "History",
  status: "active", version: 1, created_at: "2026-11-02T00:00:00Z", updated_at: "2026-11-02T00:00:00Z",
  work_item_count: 0,
}];
beforeEach(() => vi.resetAllMocks());
it("rejects late history after a session change and loads only while enabled", async () => {
  let finish!: (items: ExecutionSummary[]) => void;
  vi.mocked(getExecutionHistoryApi).mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockResolvedValue([]);
  const hook = renderHook(({ session, enabled }) => useWorkGraphHistoryResource(session, enabled), { initialProps: { session: "a", enabled: true } });
  hook.rerender({ session: "b", enabled: false });
  await act(async () => { finish(snapshot); });
  expect(hook.result.current.history).toEqual([]);
  expect(getExecutionHistoryApi).toHaveBeenCalledExactlyOnceWith("a");
  hook.rerender({ session: "b", enabled: true });
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
  expect(getExecutionHistoryApi).toHaveBeenLastCalledWith("b");
});
it("retains stale same-session history on a read failure but clears it on access loss", async () => {
  vi.mocked(getExecutionHistoryApi).mockResolvedValue(snapshot);
  const hook = renderHook(() => useWorkGraphHistoryResource("a", true));
  await waitFor(() => expect(hook.result.current.history).toEqual(snapshot));
  vi.mocked(getExecutionHistoryApi).mockRejectedValue(new Error("offline"));
  act(() => hook.result.current.refresh());
  await waitFor(() => expect(hook.result.current.isStale).toBe(true));
  expect(hook.result.current.history).toEqual(snapshot);
  vi.mocked(getExecutionHistoryApi).mockRejectedValue(new UnauthorizedError("sign in"));
  act(() => hook.result.current.refresh());
  await waitFor(() => expect(hook.result.current.history).toEqual([]));
  expect(hook.result.current.isStale).toBe(false);
});
