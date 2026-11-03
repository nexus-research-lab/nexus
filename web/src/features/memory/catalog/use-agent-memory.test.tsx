// INPUT: Real catalog hook, deferred API responses and owner/Agent changes.
// OUTPUT: Unknown deletes reconcile without replay; stale responses cannot replace a new scope.
// POS: Memory catalog command integration tests using mocked transport only.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { deleteAgentMemoryDocumentApi, getAgentMemorySnapshotApi } from "@/lib/api/agent/memory-api";
import { getWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";
import { ApiTransportError } from "@/lib/api/core/http-error";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import type { MemorySnapshot } from "@/types/memory/memory";
import { useAgentMemory } from "./use-agent-memory";

vi.mock("@/lib/api/agent/memory-api", () => ({ deleteAgentMemoryDocumentApi: vi.fn(), getAgentMemorySnapshotApi: vi.fn() }));
vi.mock("@/lib/api/agent/agent-api", () => ({ getWorkspaceFileContentApi: vi.fn() }));
const snapshot: MemorySnapshot = { layout: "topic", truncated: false, documents: [{ path: "memory/topic.md", title: "Topic", kind: "topic", indexed: true, modified_at: "2026-11-02T00:00:00Z", size: 12 }] };
const empty: MemorySnapshot = { layout: "empty", truncated: false, documents: [] };
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(getAgentMemorySnapshotApi).mockResolvedValue(snapshot);
  vi.mocked(deleteAgentMemoryDocumentApi).mockRejectedValue(new ApiTransportError("lost response", "timeout", "unknown"));
});
async function mounted() {
  const hook = renderHook(({ agentId }) => useAgentMemory(agentId, "read failed", "delete failed"), { initialProps: { agentId: "a" } });
  await waitFor(() => expect(hook.result.current.resource.isLoading).toBe(false));
  return hook;
}
async function remove(hook: Awaited<ReturnType<typeof mounted>>) {
  act(() => hook.result.current.document.requestDeleteDocument("memory/topic.md"));
  await act(() => hook.result.current.document.confirmDeleteDocument());
}
it("keeps unknown deletes locked through ordinary refresh and only explicit reconciliation clears them", async () => {
  const hook = await mounted();
  await remove(hook);
  expect(hook.result.current.document.deleteIssues).toHaveLength(1);
  expect(deleteAgentMemoryDocumentApi).toHaveBeenCalledTimes(1);
  vi.mocked(getAgentMemorySnapshotApi).mockResolvedValue(empty);
  await act(() => hook.result.current.resource.refresh());
  expect(hook.result.current.document.deleteIssues).toHaveLength(1);
  act(() => hook.result.current.document.retryDeleteDocument("memory/topic.md"));
  await act(() => hook.result.current.document.confirmDeleteDocument());
  expect(deleteAgentMemoryDocumentApi).toHaveBeenCalledTimes(1);
  await act(() => hook.result.current.document.reconcileDeleteDocument("memory/topic.md"));
  expect(hook.result.current.document.deleteIssues).toEqual([]);
  expect(deleteAgentMemoryDocumentApi).toHaveBeenCalledTimes(1);
});
it("does not downgrade a committed delete when the subsequent directory read fails", async () => {
  const hook = await mounted();
  vi.mocked(deleteAgentMemoryDocumentApi).mockResolvedValue(undefined);
  vi.mocked(getAgentMemorySnapshotApi).mockRejectedValue(new Error("offline"));
  await remove(hook);
  expect(hook.result.current.document.deleteIssues[0]).toMatchObject({ kind: "committed_refresh", directoryCheck: "failed" });
  act(() => hook.result.current.document.retryDeleteDocument("memory/topic.md"));
  await act(() => hook.result.current.document.confirmDeleteDocument());
  expect(deleteAgentMemoryDocumentApi).toHaveBeenCalledTimes(1);
  vi.mocked(getAgentMemorySnapshotApi).mockResolvedValue(empty);
  await act(() => hook.result.current.document.reconcileDeleteDocument("memory/topic.md"));
  expect(hook.result.current.document.deleteIssues).toEqual([]);
});
it("uses an exact file read when a truncated directory cannot prove deletion", async () => {
  const hook = await mounted();
  vi.mocked(getAgentMemorySnapshotApi).mockResolvedValue({ ...empty, truncated: true });
  vi.mocked(getWorkspaceFileContentApi).mockRejectedValue(new Error("read unavailable"));
  await remove(hook);
  expect(getWorkspaceFileContentApi).toHaveBeenCalledWith("a", "memory/topic.md");
  expect(hook.result.current.document.deleteIssues[0].directoryCheck).toBe("failed");
  expect(deleteAgentMemoryDocumentApi).toHaveBeenCalledTimes(1);
});
it.each(["agent", "owner"])("discards a late delete response after the %s changes", async (change) => {
  const hook = await mounted();
  let reject!: (error: Error) => void;
  vi.mocked(deleteAgentMemoryDocumentApi).mockReturnValue(new Promise((_, fail) => { reject = fail; }));
  act(() => hook.result.current.document.requestDeleteDocument("memory/topic.md"));
  let pending!: Promise<void>;
  act(() => { pending = hook.result.current.document.confirmDeleteDocument(); });
  vi.mocked(getAgentMemorySnapshotApi).mockResolvedValue(empty);
  act(() => {
    if (change === "agent") hook.rerender({ agentId: "b" });
    else { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); }
  });
  await waitFor(() => expect(hook.result.current.resource.snapshot).toEqual(empty));
  await act(async () => { reject(new Error("late failure")); await pending; });
  expect(hook.result.current.document.deleteIssues).toEqual([]);
  expect(hook.result.current.resource.snapshot).toEqual(empty);
  expect(getAgentMemorySnapshotApi).toHaveBeenCalledTimes(2);
});
