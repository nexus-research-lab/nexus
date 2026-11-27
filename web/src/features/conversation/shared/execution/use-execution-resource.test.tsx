// INPUT: 连续 execution_invalidated revision、在途读取与页面可见性变化。
// OUTPUT: 同一 session 同时至多一个读取、在途失效只补读一次、隐藏期间推迟到可见时读取。
// POS: WorkGraph 骨架资源的读取调度回归；访问失效与历史隔离见 execution-resource-access.test.tsx。
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ExecutionView } from "@/types/conversation/execution";
import { useExecutionResource } from "./use-execution-resource";

const api = vi.hoisted(() => ({ latest: vi.fn() }));
vi.mock("@/lib/api/conversation/execution-api", () => ({ getLatestExecutionApi: api.latest }));

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

let visibility: DocumentVisibilityState = "visible";
beforeEach(() => {
  vi.useFakeTimers();
  visibility = "visible";
  vi.spyOn(document, "visibilityState", "get").mockImplementation(() => visibility);
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.clearAllMocks();
  vi.useRealTimers();
});

it("coalesces invalidations during an in-flight read into one trailing read", async () => {
  const first = deferred<ExecutionView | null>();
  const second = deferred<ExecutionView | null>();
  api.latest.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
  const view = renderHook(
    ({ revision }) => useExecutionResource({ invalidationKey: revision, sessionKey: "session" }),
    { initialProps: { revision: 0 } },
  );
  await act(async () => { vi.advanceTimersByTime(200); });
  expect(api.latest).toHaveBeenCalledTimes(1);

  // 工具事件连续失效：在途期间不再并发读取。
  for (let revision = 1; revision <= 5; revision += 1) {
    view.rerender({ revision });
    await act(async () => { vi.advanceTimersByTime(200); });
  }
  expect(api.latest).toHaveBeenCalledTimes(1);

  const stale = { id: "graph", status: "active" } as ExecutionView;
  await act(async () => { first.resolve(stale); });
  expect(view.result.current.execution).toBe(stale);
  expect(api.latest).toHaveBeenCalledTimes(2);

  const latest = { id: "graph", status: "completed" } as ExecutionView;
  await act(async () => { second.resolve(latest); });
  expect(view.result.current.execution).toBe(latest);
  expect(api.latest).toHaveBeenCalledTimes(2);
});

it("defers reads while hidden and reads once when the page becomes visible", async () => {
  const graph = { id: "graph", status: "active" } as ExecutionView;
  api.latest.mockResolvedValue(graph);
  visibility = "hidden";
  const view = renderHook(
    ({ revision }) => useExecutionResource({ invalidationKey: revision, sessionKey: "session" }),
    { initialProps: { revision: 0 } },
  );
  for (let revision = 1; revision <= 3; revision += 1) {
    view.rerender({ revision });
    await act(async () => { vi.advanceTimersByTime(200); });
  }
  expect(api.latest).not.toHaveBeenCalled();

  visibility = "visible";
  await act(async () => { document.dispatchEvent(new Event("visibilitychange")); });
  expect(api.latest).toHaveBeenCalledTimes(1);
  expect(view.result.current.execution).toBe(graph);
});

it("starts the new session read immediately instead of waiting for the old one", async () => {
  const old = deferred<ExecutionView | null>();
  const next = { id: "next", status: "active" } as ExecutionView;
  api.latest.mockReturnValueOnce(old.promise).mockResolvedValueOnce(next);
  const view = renderHook(
    ({ sessionKey }) => useExecutionResource({ sessionKey }),
    { initialProps: { sessionKey: "old" } },
  );
  await act(async () => { vi.advanceTimersByTime(200); });
  view.rerender({ sessionKey: "new" });
  await act(async () => { vi.advanceTimersByTime(200); });
  expect(api.latest).toHaveBeenLastCalledWith("new");
  expect(view.result.current.execution).toBe(next);
  await act(async () => { old.resolve({ id: "old", status: "active" } as ExecutionView); });
  expect(view.result.current.execution).toBe(next);
});
