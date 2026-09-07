// INPUT: 保存草图表单、编辑器受理回调与数据库提交回执。
// OUTPUT: 验证改名直接保存、只传显式表单修改、关闭重开与失败重开的元信息边界。
// POS: WorkGraph 保存确认表单 DOM 回归；模型编辑和画布由独立边界替身代替。

import { act, render, screen, waitFor } from "@testing-library/react";
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
  mocks.save.mockReset().mockResolvedValue({ preview_id: PREVIEW.preview_id, status: "saved", workflow: { ...PREVIEW, slash_name: "chain", id: "workflow-a", version: 1 } });
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
    await act(async () => completeSave({
      preview_id: PREVIEW.preview_id, status: "saved", workflow: { ...PREVIEW, slash_name: "research-rich" },
    }));
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    expect(screen.getByLabelText<HTMLInputElement>("execution.workflow_slash_name").value).toBe("research-rich");
    expect(dispatch).toHaveBeenCalledWith(refresh);
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
    mocks.state.mockResolvedValueOnce({ preview: PREVIEW, workflow: persisted, status: "saved" });
    await user.click(check);
    expect(screen.getByText("execution.workflow_saved_title")).toBeTruthy();
    expect(onSaved).toHaveBeenCalledExactlyOnceWith(persisted);
    expect(mocks.save).toHaveBeenCalledTimes(1);
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
});
