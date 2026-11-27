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
