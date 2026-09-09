// INPUT: 保存草图表单、编辑器受理回调与数据库提交回执。
// OUTPUT: 验证连续改名保存、已保存内容判定、当前版本续用，以及编辑器显式元信息边界。
// POS: WorkGraph 保存确认表单 DOM 回归；模型编辑和画布由独立边界替身代替。

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/lib/conversation/workgraph-workflow-events";
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { WorkGraphDistillationDialog } from "./workgraph-distillation-dialog";

const mocks = vi.hoisted(() => ({ save: vi.fn(), state: vi.fn(), metadata: vi.fn() }));
vi.mock("@/shared/i18n/i18n-context", () => ({
  useI18n: () => ({ locale: "zh", t: (key: string) => key }),
}));
vi.mock("@/lib/api/conversation/execution-api", () => ({
  saveWorkGraphWorkflowApi: mocks.save,
  getWorkGraphWorkflowSaveStateApi: mocks.state,
}));
vi.mock("./use-workgraph-slash-name-availability", () => ({
  useWorkGraphSlashNameAvailability: ({ slashName }: { slashName: string }) => ({ slashName, status: "available" }),
}));
vi.mock("./workgraph-workflow-canvas-preview", () => ({ WorkGraphWorkflowCanvasPreview: () => null }));
vi.mock("./workgraph-metadata-editor-dialog", () => ({
  WorkGraphMetadataEditorDialog: ({ metadata, onClose, onMetadataApplied }: {
    metadata: unknown;
    onClose: () => void;
    onMetadataApplied: (preview: WorkGraphWorkflowPreview) => void;
  }) => {
    mocks.metadata(metadata);
    return <div>
      <button onClick={() => onMetadataApplied({ ...PREVIEW, ...(metadata as object), head_revision: 2, selected_revision: 2 })}>Accept form edits</button>
      <button onClick={onClose}>Close editor</button>
    </div>;
  },
}));

const PREVIEW: WorkGraphWorkflowPreview = {
  head_revision: 1, selected_revision: 1,
  preview_id: "preview-a", slash_name: "report", title: "报告", description: "生成报告",
  source_execution_id: "execution-a", source_session_key: "session-a",
  objective: "生成报告", nodes: [], expires_at: "2026-10-01T00:00:00Z",
};
beforeEach(() => {
  mocks.state.mockReset().mockResolvedValue({ preview: PREVIEW, status: "unsaved", saved_revision: 0 });
  mocks.save.mockReset().mockImplementation(async (_session, _preview, metadata) => {
    const preview = { ...PREVIEW, ...metadata, head_revision: metadata.head_revision + 1, selected_revision: metadata.head_revision + 1 };
    const workflow = { ...preview, id: "workflow-a", version: metadata.head_revision };
    mocks.state.mockResolvedValue({ preview, workflow, status: "saved", saved_revision: preview.selected_revision });
    return { preview_id: preview.preview_id, status: "saved", workflow };
  });
  mocks.metadata.mockClear();
});
afterEach(() => vi.restoreAllMocks());

describe("WorkGraph save form metadata", () => {
  it("saves the normalized name the user typed", async () => {
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    const name = screen.getByLabelText("execution.workflow_slash_name");
    await waitFor(() => expect((name as HTMLInputElement).disabled).toBe(false));
    await user.clear(name);
    await user.type(name, "/Chain");
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(mocks.save).toHaveBeenCalledExactlyOnceWith("session-a", "preview-a", {
      head_revision: 1, selected_revision: 1,
      slash_name: "chain", title: "报告", description: "生成报告",
    });
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
  });

  it("waits for the committed receipt before showing success and refreshing commands", async () => {
    let completeSave: (value: unknown) => void = () => {};
    mocks.save.mockReturnValue(new Promise((resolve) => { completeSave = resolve; }));
    const dispatch = vi.spyOn(window, "dispatchEvent");
    const refresh = expect.objectContaining({ type: WORKGRAPH_WORKFLOWS_CHANGED_EVENT });
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(screen.queryByText("execution.workflow_saved_title")).toBeNull();
    expect(dispatch).not.toHaveBeenCalledWith(refresh);
    const persistedPreview = { ...PREVIEW, slash_name: "research-rich", head_revision: 2, selected_revision: 2 };
    mocks.state.mockResolvedValue({ preview: persistedPreview, workflow: persistedPreview, status: "saved", saved_revision: 2 });
    await act(async () => completeSave({
      preview_id: PREVIEW.preview_id, status: "saved", workflow: { ...PREVIEW, slash_name: "research-rich" },
    }));
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    expect(screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name").value).toBe("research-rich");
    expect(dispatch).toHaveBeenCalledWith(refresh);
  });

  it("keeps the saved form editable and uses the committed revision for the next save", async () => {
    const onSaved = vi.fn();
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} onSaved={onSaved} preview={PREVIEW} sessionKey="session-a" />);
    const name = screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name");
    await waitFor(() => expect(name.disabled).toBe(false));
    await user.clear(name);
    await user.type(name, "chain");
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    await screen.findByText("execution.workflow_saved_title");
    expect(name.disabled).toBe(false);
    expect(screen.getByRole<HTMLButtonElement>("button", { name: "execution.workflow_edit_with_chat" }).disabled).toBe(false);
    await user.clear(name);
    await user.type(name, "research-rich");
    expect(screen.queryByText("execution.workflow_saved_title")).toBeNull();
    const title = screen.getByLabelText("execution.workflow_title");
    await user.clear(title);
    await user.type(title, "新标题");
    const description = screen.getByLabelText("execution.workflow_description");
    await user.clear(description);
    await user.type(description, "新描述");
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(mocks.save).toHaveBeenNthCalledWith(2, "session-a", "preview-a", {
      head_revision: 2, selected_revision: 2, slash_name: "research-rich", title: "新标题", description: "新描述",
    });
    await screen.findByText("execution.workflow_saved_title");
    expect(name.disabled).toBe(false);
    expect(onSaved).toHaveBeenCalledTimes(2);
    expect(onSaved.mock.calls.map(([workflow]) => workflow.id)).toEqual(["workflow-a", "workflow-a"]);
  });

  it("recognizes a reopened saved graph and restores saved status when metadata edits are reverted", async () => {
    mocks.state.mockResolvedValue({ preview: PREVIEW, workflow: { ...PREVIEW, id: "workflow-a" }, status: "saved", saved_revision: 1 });
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    await screen.findByText("execution.workflow_saved_title");
    for (const [label, value] of [["execution.workflow_slash_name", "report"], ["execution.workflow_title", "报告"], ["execution.workflow_description", "生成报告"]]) {
      const field = screen.getByLabelText(label);
      await user.type(field, "a");
      expect(screen.queryByText("execution.workflow_saved_title")).toBeNull();
      expect(screen.getByRole<HTMLButtonElement>("button", { name: "execution.workflow_save_sketch" }).disabled).toBe(false);
      await user.clear(field);
      await user.type(field, value);
      expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    }
    expect(mocks.save).not.toHaveBeenCalled();
  });

  it("preserves committed success when refreshing the saved Draft fails and resumes after a read", async () => {
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    const name = screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name");
    await waitFor(() => expect(name.disabled).toBe(false));
    mocks.state.mockRejectedValueOnce(new Error("read failed after commit"));
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    await screen.findByText("execution.workflow_state_failed");
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    expect(name.disabled).toBe(true);
    expect(screen.queryByRole("button", { name: "execution.workflow_check_save" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "execution.workflow_reload_draft" }));
    expect(name.disabled).toBe(false);
    expect(mocks.save).toHaveBeenCalledTimes(1);
  });

  it("does not treat a legacy scheduled response as a completed save", async () => {
    mocks.save.mockResolvedValue({ preview_id: PREVIEW.preview_id, status: "scheduled" });
    const dispatch = vi.spyOn(window, "dispatchEvent");
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(screen.queryByText("execution.workflow_saved_title")).toBeNull();
    expect(dispatch).not.toHaveBeenCalledWith(expect.objectContaining({ type: WORKGRAPH_WORKFLOWS_CHANGED_EVENT }));
  });

  it("resumes without overwriting chat metadata and sends only subsequent form edits", async () => {
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    const open = () => user.click(screen.getByRole("button", { name: "execution.workflow_edit_with_chat" }));
    await open();
    expect(mocks.metadata).toHaveBeenLastCalledWith({});
    await user.click(screen.getByRole("button", { name: "Close editor" }));
    const name = screen.getByLabelText("execution.workflow_slash_name");
    await waitFor(() => expect((name as HTMLInputElement).disabled).toBe(false));
    await user.clear(name);
    await user.type(name, "chain");
    await open();
    expect(mocks.metadata).toHaveBeenLastCalledWith({ slash_name: "chain" });
    await user.click(screen.getByRole("button", { name: "Accept form edits" }));
    await user.click(screen.getByRole("button", { name: "Close editor" }));
    await open();
    expect(mocks.metadata).toHaveBeenLastCalledWith({});
    expect(mocks.save).not.toHaveBeenCalled();
  });

  it("keeps explicit edits for retry when opening the editor was not accepted", async () => {
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    const name = screen.getByLabelText("execution.workflow_slash_name");
    await waitFor(() => expect((name as HTMLInputElement).disabled).toBe(false));
    await user.clear(name);
    await user.type(name, "chain");
    await user.click(screen.getByRole("button", { name: "execution.workflow_edit_with_chat" }));
    await user.click(screen.getByRole("button", { name: "Close editor" }));
    await user.click(screen.getByRole("button", { name: "execution.workflow_edit_with_chat" }));
    expect(mocks.metadata).toHaveBeenLastCalledWith({ slash_name: "chain" });
  });

  it("loads the current Draft before confirming an older card and keeps the active command separate", async () => {
    mocks.state.mockResolvedValue({
      preview: { ...PREVIEW, slash_name: "chain", head_revision: 2, selected_revision: 2 },
      workflow: { ...PREVIEW, id: "workflow-a", version: 1 }, status: "unsaved", saved_revision: 1,
    });
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    await waitFor(() => expect(screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name").value).toBe("chain"));
    expect(screen.getByText("execution.workflow_current_command")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(mocks.save).toHaveBeenCalledExactlyOnceWith("session-a", "preview-a", {
      head_revision: 2, selected_revision: 2, slash_name: "chain", title: "报告", description: "生成报告",
    });
  });

  it("checks a lost response against persisted content without repeating the save", async () => {
    mocks.save.mockRejectedValue(new Error("response lost"));
    const onSaved = vi.fn();
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} onSaved={onSaved} preview={PREVIEW} sessionKey="session-a" />);
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    const check = screen.getByRole("button", { name: "execution.workflow_check_save" });
    // The same name alone cannot prove this exact graph was saved.
    mocks.state.mockResolvedValueOnce({ preview: PREVIEW, workflow: { ...PREVIEW, objective: "a different graph" }, status: "saved" });
    await user.click(check);
    expect(screen.queryByText("execution.workflow_saved_title")).toBeNull();
    expect(onSaved).not.toHaveBeenCalled();
    const persisted = { ...PREVIEW, id: "workflow-a", version: 2 };
    mocks.state.mockResolvedValueOnce({ preview: { ...PREVIEW, head_revision: 3, selected_revision: 2 }, workflow: persisted, status: "saved" });
    await user.click(check);
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    expect(onSaved).toHaveBeenCalledExactlyOnceWith(persisted);
    expect(mocks.save).toHaveBeenCalledTimes(1);
    const name = screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name");
    expect(name.disabled).toBe(false);
    await user.clear(name);
    await user.type(name, "after-check");
    await user.click(screen.getByRole("button", { name: "execution.workflow_save_sketch" }));
    expect(mocks.save).toHaveBeenNthCalledWith(2, "session-a", "preview-a", {
      head_revision: 3, selected_revision: 2, slash_name: "after-check", title: "报告", description: "生成报告",
    });
  });

  it("keeps save disabled when current Draft state cannot be loaded", async () => {
    mocks.state.mockRejectedValueOnce(new Error("read failed"));
    const user = userEvent.setup();
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    await screen.findByText("execution.workflow_state_failed");
    const save = screen.getByRole<HTMLButtonElement>("button", { name: "execution.workflow_save_sketch" });
    expect(save.disabled).toBe(true);
    expect(mocks.save).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "execution.workflow_reload_draft" }));
    expect(save.disabled).toBe(false);
  });
  it("blocks same-frame duplicate saves and reports failed verification without unlocking", async () => {
    let reject!: (reason: Error) => void;
    mocks.save.mockReturnValue(new Promise((_resolve, fail) => { reject = fail; }));
    render(<WorkGraphDistillationDialog agents={[]} onClose={vi.fn()} preview={PREVIEW} sessionKey="session-a" />);
    const button = screen.getByRole<HTMLButtonElement>("button", { name: "execution.workflow_save_sketch" });
    await waitFor(() => expect(button.disabled).toBe(false));
    act(() => { fireEvent.click(button); fireEvent.click(button); });
    expect(mocks.save).toHaveBeenCalledOnce();
    await act(async () => reject(new Error("lost response")));
    mocks.state.mockRejectedValueOnce(new Error("offline"));
    await userEvent.setup().click(screen.getByRole("button", { name: "execution.workflow_check_save" }));
    expect(screen.getByText("execution.workflow_state_failed")).toBeTruthy();
    expect(screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name").disabled).toBe(true);
    expect(mocks.save).toHaveBeenCalledOnce();
  });
});
