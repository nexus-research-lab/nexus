// INPUT: Scoped execution snapshots, access revocation and late reads.
// OUTPUT: Revoked graphs are discarded; departed scopes cannot publish late history.
// POS: Execution read boundary regression.
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api/core/http-error";
import type { ExecutionView } from "@/types/conversation/execution";
import { useExecutionResource } from "./use-execution-resource";
import { useWorkGraphHistoryResource } from "./use-workgraph-history-resource";
const api = vi.hoisted(() => ({ latest: vi.fn(), history: vi.fn() }));
vi.mock("@/lib/api/conversation/execution-api", () => ({ getLatestExecutionApi: api.latest, getExecutionHistoryApi: api.history }));
afterEach(() => { vi.clearAllMocks(); vi.useRealTimers(); });
const execution = { id: "graph", status: "completed" } as ExecutionView;
it("discards both current and historical snapshots on access denial", async () => {
  api.latest.mockResolvedValueOnce(execution).mockRejectedValueOnce(new ApiRequestError("secret", 403));
  api.history.mockResolvedValueOnce([execution]).mockRejectedValueOnce(new ApiRequestError("secret", 403));
  const latest = renderHook(() => useExecutionResource({ sessionKey: "session" }));
  const history = renderHook(() => useWorkGraphHistoryResource("session", true));
  await waitFor(() => expect(latest.result.current.execution).toBe(execution));
  expect(history.result.current.history).toEqual([execution]);
  await act(async () => { latest.result.current.refresh(); history.result.current.refresh(); });
  expect(latest.result.current.execution).toBeNull();
  expect(history.result.current.history).toEqual([]);
  api.latest.mockReturnValue(new Promise(() => {}));
  api.history.mockReturnValue(new Promise(() => {}));
  act(() => { latest.result.current.refresh(); history.result.current.refresh(); });
  expect(latest.result.current.execution).toBeNull();
  expect(history.result.current.history).toEqual([]);
});
it("invalidates historical reads when the surface leaves and returns to the same session", async () => {
  let resolve!: (value: ExecutionView[]) => void;
  api.history.mockReturnValueOnce(new Promise<ExecutionView[]>((done) => { resolve = done; })).mockReturnValue(new Promise(() => {}));
  const view = renderHook(({ enabled }) => useWorkGraphHistoryResource("session", enabled), { initialProps: { enabled: true } });
  view.rerender({ enabled: false });
  view.rerender({ enabled: true });
  await act(async () => resolve([execution]));
  expect(view.result.current.history).toEqual([]);
});
