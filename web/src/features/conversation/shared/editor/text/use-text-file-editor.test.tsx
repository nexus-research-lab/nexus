// INPUT: Text editor hook with revisioned reads, failed writes and changing scopes.
// OUTPUT: Unread files cannot save; unknown writes reconcile read-only and preserve newer drafts.
// POS: Workspace editor recovery integration tests; transport alone is mocked.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { getWorkspaceFileContentApi, updateWorkspaceFileContentApi } from "@/lib/api/agent/agent-api";
import { ApiTransportError } from "@/lib/api/core/http-error";
import { useTextFileEditor } from "./use-text-file-editor";
vi.mock("@/lib/api/agent/agent-api", () => ({ getWorkspaceFileContentApi: vi.fn(), updateWorkspaceFileContentApi: vi.fn() }));
const baseline = { path: "notes.md", content: "original", revision: "r1" };
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(getWorkspaceFileContentApi).mockResolvedValue(baseline);
});
function mount() { return renderHook(({ agentId, path }) => useTextFileEditor({ agentId, path, fallbackLoadError: "load failed", fallbackSaveError: "save failed" }), { initialProps: { agentId: "a", path: "notes.md" } }); }
it("cannot edit or save an unread file", async () => {
  vi.mocked(getWorkspaceFileContentApi).mockRejectedValue(new Error("offline"));
  const hook = mount();
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false));
  act(() => { hook.result.current.setIsEditing(true); hook.result.current.setDraftContent("draft"); });
  await act(() => hook.result.current.save().then(saved => { expect(saved).toBe(false); }));
  expect(hook.result.current.isEditing).toBe(false);
  expect(updateWorkspaceFileContentApi).not.toHaveBeenCalled();
});
it("sends the loaded revision once and reconciles unknown outcomes without replay", async () => {
  const hook = mount();
  await waitFor(() => expect(hook.result.current.hasLoadedContent).toBe(true));
  act(() => { hook.result.current.setIsEditing(true); hook.result.current.setDraftContent("submitted"); });
  vi.mocked(updateWorkspaceFileContentApi).mockRejectedValue(new ApiTransportError("lost", "timeout", "unknown"));
  await act(async () => { await hook.result.current.save(); });
  expect(updateWorkspaceFileContentApi).toHaveBeenCalledExactlyOnceWith("a", "notes.md", "submitted", "r1");
  expect(hook.result.current.saveIssue?.kind).toBe("outcome_unknown");
  act(() => hook.result.current.setDraftContent("newer typing"));
  await act(async () => { await hook.result.current.save(); });
  expect(updateWorkspaceFileContentApi).toHaveBeenCalledTimes(1);
  vi.mocked(getWorkspaceFileContentApi).mockResolvedValue({ ...baseline, content: "submitted", revision: "r2" });
  await act(() => hook.result.current.reconcileSave());
  expect(hook.result.current.revision).toBe("r2");
  expect(hook.result.current.draftContent).toBe("newer typing");
  expect(hook.result.current.saveIssue).toBeNull();
  expect(updateWorkspaceFileContentApi).toHaveBeenCalledTimes(1);
});
it("does not publish a late save into another Agent or path", async () => {
  const hook = mount();
  await waitFor(() => expect(hook.result.current.hasLoadedContent).toBe(true));
  act(() => { hook.result.current.setIsEditing(true); hook.result.current.setDraftContent("submitted"); });
  let finish!: (value: typeof baseline) => void;
  vi.mocked(updateWorkspaceFileContentApi).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  let pending!: Promise<boolean>;
  act(() => { pending = hook.result.current.save(); });
  vi.mocked(getWorkspaceFileContentApi).mockResolvedValue({ path: "other.md", content: "other document", revision: "other-r1" });
  hook.rerender({ agentId: "b", path: "other.md" });
  await waitFor(() => expect(hook.result.current.draftContent).toBe("other document"));
  await act(async () => { finish({ ...baseline, content: "submitted", revision: "r2" }); await pending; });
  expect(hook.result.current.draftContent).toBe("other document");
  expect(hook.result.current.revision).toBe("other-r1");
});
