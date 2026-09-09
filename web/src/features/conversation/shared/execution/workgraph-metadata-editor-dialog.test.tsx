// INPUT: 隐藏草图编辑会话的版本切换、异步刷新与应用结果。
// OUTPUT: 验证切换期间禁止应用，迟到读取不回退版本，冲突后先刷新再显式应用。
// POS: WorkGraph 编辑器 DOM 回归；聊天与画布由独立边界替身代替。

import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiRequestError } from "@/lib/api/core/http-error";
import type { Agent } from "@/types/agent/agent";
import type { WorkGraphWorkflowEditorSession, WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { WorkGraphMetadataEditorDialog } from "./workgraph-metadata-editor-dialog";

const mocks = vi.hoisted(() => ({ start: vi.fn(), get: vi.fn(), select: vi.fn(), apply: vi.fn() }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ locale: "zh", t: (key: string) => key }) }));
vi.mock("@/hooks/settings/use-default-agent-runtime-kind", () => ({ useDefaultAgentRuntimeKind: () => "nxs" }));
vi.mock("@/store/agent", () => ({
  useAgentStore: (selector: (state: unknown) => unknown) => selector({ agents: [], load_agents_from_server: mocks.get }),
}));
vi.mock("@/lib/api/conversation/execution-api", () => ({
  startWorkGraphWorkflowEditorApi: mocks.start, getWorkGraphWorkflowEditorApi: mocks.get,
  selectWorkGraphWorkflowEditorVersionApi: mocks.select, applyWorkGraphWorkflowEditorApi: mocks.apply,
}));
vi.mock("@/features/conversation/room/dm/panel/dm-chat-panel", () => ({
  DmChatPanel: ({ onConversationSnapshotChange }: { onConversationSnapshotChange: () => void }) => <button onClick={onConversationSnapshotChange}>Chat snapshot</button>,
}));
vi.mock("./execution-workgraph-canvas", () => ({ ExecutionWorkGraphCanvas: () => null }));

const AGENT = { agent_id: "editor-agent" } as Agent;
const PREVIEW: WorkGraphWorkflowPreview = {
  preview_id: "preview-a", head_revision: 2, selected_revision: 1,
  slash_name: "report", title: "报告", description: "生成报告", objective: "报告",
  source_execution_id: "execution-a", source_session_key: "session-a", nodes: [], expires_at: "2026-10-01T00:00:00Z",
};
function editor(selectedRevision: number): WorkGraphWorkflowEditorSession {
  return {
    editor_id: "editor-a", agent_id: AGENT.agent_id, session_key: "editor-session", revision: 2,
    selected_revision: selectedRevision, display_after_unix_milli: 0, expires_at: PREVIEW.expires_at,
    preview: { ...PREVIEW, selected_revision: selectedRevision, slash_name: selectedRevision === 1 ? "report" : "chain" },
    versions: [1, 2].map(revision => ({ revision, selected: revision === selectedRevision, slash_name: "report", title: "报告", node_count: 0, dependency_count: 0, created_at: "2026-09-07T00:00:00Z" })),
  };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(complete => { resolve = complete; });
  return { promise, resolve };
}
beforeEach(() => {
  mocks.start.mockReset().mockResolvedValue(editor(1));
  mocks.get.mockReset().mockResolvedValue(editor(1));
  mocks.select.mockReset().mockResolvedValue(editor(2));
  mocks.apply.mockReset().mockResolvedValue(editor(2).preview);
});
async function openEditor() {
  const onApply = vi.fn();
  render(<WorkGraphMetadataEditorDialog agents={[AGENT]} onApply={onApply} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
  await screen.findByRole("button", { name: "Chat snapshot" });
  return { user: userEvent.setup(), onApply, apply: screen.getByRole<HTMLButtonElement>("button", { name: "execution.workflow_editor_apply" }) };
}

describe("WorkGraph editor version ordering", () => {
  it("waits for version selection before applying its exact preview", async () => {
    const selection = deferred<WorkGraphWorkflowEditorSession>();
    mocks.select.mockReturnValue(selection.promise);
    const { user, onApply, apply } = await openEditor();
    await user.click(screen.getByRole("button", { name: "v2" }));
    expect(apply.disabled).toBe(true);
    await user.click(apply);
    expect(mocks.apply).not.toHaveBeenCalled();
    await act(async () => selection.resolve(editor(2)));
    expect(apply.disabled).toBe(false);
    await user.click(apply);
    expect(mocks.apply).toHaveBeenCalledExactlyOnceWith("session-a", "editor-a", 2, 2);
    expect(onApply).toHaveBeenCalledExactlyOnceWith(editor(2).preview);
  });

  it("ignores a read captured before a newer selection completed", async () => {
    const staleRead = deferred<WorkGraphWorkflowEditorSession>();
    mocks.get.mockReturnValueOnce(staleRead.promise);
    const { user, apply } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Chat snapshot" }));
    await user.click(screen.getByRole("button", { name: "v2" }));
    await act(async () => staleRead.resolve(editor(1)));
    await user.click(apply);
    expect(mocks.apply).toHaveBeenCalledExactlyOnceWith("session-a", "editor-a", 2, 2);
  });

  it("blocks stale actions after a failed read until an explicit refresh succeeds", async () => {
    mocks.get.mockRejectedValueOnce(new Error("offline"));
    const { user, apply } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Chat snapshot" }));
    expect(apply.disabled).toBe(true);
    expect(screen.getByRole<HTMLButtonElement>("button", { name: "v2" }).disabled).toBe(true);
    await user.click(screen.getByRole("button", { name: "execution.workflow_editor_refresh_action" }));
    expect(apply.disabled).toBe(false);
    expect(mocks.apply).not.toHaveBeenCalled();
  });

  it("recovers a rejected stale application by reading before the next explicit apply", async () => {
    mocks.apply.mockRejectedValueOnce(new ApiRequestError("changed", 412, {
      version: 1, code: "conflict", category: "conflict", effect: "not_applied",
    }));
    const { user, apply } = await openEditor();
    await user.click(apply);
    mocks.get.mockResolvedValueOnce(editor(2));
    await user.click(screen.getByRole("button", { name: "execution.workflow_editor_refresh_action" }));
    expect(mocks.apply).toHaveBeenCalledTimes(1);
    await user.click(apply);
    expect(mocks.apply).toHaveBeenNthCalledWith(2, "session-a", "editor-a", 2, 2);
  });
  it("uses a synchronous version gate and does not read over an in-flight selection", async () => {
    const selection = deferred<WorkGraphWorkflowEditorSession>();
    mocks.select.mockReturnValue(selection.promise);
    await openEditor();
    const version = screen.getByRole("button", { name: "v2" });
    act(() => { fireEvent.click(version); fireEvent.click(version); fireEvent.click(screen.getByRole("button", { name: "Chat snapshot" })); });
    expect(mocks.select).toHaveBeenCalledOnce();
    expect(mocks.get).not.toHaveBeenCalled();
    await act(async () => selection.resolve(editor(2)));
  });
  it("hides editor content after access revocation until a successful read", async () => {
    mocks.get.mockRejectedValueOnce(new ApiRequestError("secret", 403));
    const { user } = await openEditor();
    await user.click(screen.getByRole("button", { name: "Chat snapshot" }));
    expect(screen.queryByRole("button", { name: "Chat snapshot" })).toBeNull();
    expect(screen.queryByText("execution.workflow_editor_apply")).toBeNull();
    await user.click(screen.getByRole("button", { name: "state.retry" }));
    expect(screen.getByRole("button", { name: "Chat snapshot" })).toBeTruthy();
  });
});
