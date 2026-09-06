// INPUT: Scoped Memory controller states at the presentation boundary.
// OUTPUT: Named loading, access-failure escape and explicit conflict/unknown-save actions.
// POS: Real document surface regression; no file request or mutation is performed.

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { MemoryDocumentPanel } from "./memory-document-panel";
import { useMemoryDocument } from "./use-memory-document";

vi.mock("./use-memory-document", () => ({ useMemoryDocument: vi.fn() }));
vi.mock("@/hooks/agent/use-workspace-markdown", () => ({ useWorkspaceMarkdown: () => ({
  resolveFilePath: vi.fn(), getFilePreviewUrl: vi.fn(),
}) }));

let controller: ReturnType<typeof useMemoryDocument>;
beforeEach(() => {
  controller = {
    scopeKey: "agent:memory/project.md", content: "Saved version", draft: "My draft", revision: "revision-1",
    command: null, commandError: null, editing: true, isLoading: false, resourceError: null, saveIssue: null,
    dirty: true, isReconciling: false, isSaving: false, saveBlocked: false,
    adoptLatest: vi.fn(), cancelEditing: vi.fn(), setDraft: vi.fn(), startEditing: vi.fn(),
    reload: vi.fn(async () => undefined), save: vi.fn(async () => undefined),
    overwriteConflict: vi.fn(async () => undefined), reconcileSave: vi.fn(async () => undefined),
  };
  vi.mocked(useMemoryDocument).mockImplementation(() => controller);
});

function panel() {
  const onBack = vi.fn();
  const onDelete = vi.fn();
  const result = render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}>
    <MemoryDocumentPanel agentId="agent" deleteBusy={false} deleting={false}
      document={{ kind: "topic", indexed: true, modified_at: new Date().toISOString(), path: "memory/project.md", size: 20, title: "Project" }}
      onBack={onBack} onDelete={onDelete} onSaved={vi.fn()} onSelectPath={vi.fn()} />
  </I18N_CONTEXT.Provider>);
  return { ...result, onBack, onDelete };
}

it("announces initial document loading instead of an unnamed spinner", () => {
  controller.content = ""; controller.isLoading = true; controller.editing = false;
  panel();
  const loading = screen.getByText("Loading…").closest('[role="status"]');
  expect(loading?.getAttribute("aria-busy")).toBe("true");
  expect(screen.queryByRole("textbox")).toBeNull();
});

it("retains a compact return action on access failure without exposing the document or editing actions", async () => {
  controller.resourceError = { access: "forbidden", message: "denied" };
  const { onBack, onDelete } = panel();
  expect(screen.queryByText("Project")).toBeNull();
  expect(screen.queryByRole("textbox")).toBeNull();
  expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Back to memory list" }));
  expect(onBack).toHaveBeenCalledOnce();
  expect(controller.reload).not.toHaveBeenCalled();
  expect(onDelete).not.toHaveBeenCalled();
});

it("keeps both conflict versions and requires an explicit overwrite action", async () => {
  controller.saveIssue = { kind: "conflict", phase: "review" }; controller.saveBlocked = true;
  panel();
  const draft = screen.getByRole("textbox", { name: MESSAGES.en["capability.memory_local_draft"] });
  const saved = screen.getByRole("textbox", { name: MESSAGES.en["capability.memory_saved_version"] }) as HTMLTextAreaElement;
  expect(saved.readOnly).toBe(true);
  expect(saved.value).toBe("Saved version");
  fireEvent.change(draft, { target: { value: "Still editing" } });
  expect(controller.setDraft).toHaveBeenCalledExactlyOnceWith("Still editing");
  expect(controller.overwriteConflict).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["capability.memory_overwrite_draft"] }));
  expect(controller.overwriteConflict).toHaveBeenCalledOnce();
  expect(controller.save).not.toHaveBeenCalled();
});

it("only reconciles an unknown save while retaining the editable draft", async () => {
  controller.saveIssue = { kind: "outcome_unknown", attemptedDraft: "My draft", expectedRevision: "revision-1", reconciliationFailed: false };
  controller.saveBlocked = true;
  panel();
  expect((screen.getByRole("button", { name: "Save" }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("My draft");
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["capability.memory_check_save_result"] }));
  expect(controller.reconcileSave).toHaveBeenCalledOnce();
  expect(controller.save).not.toHaveBeenCalled();
  expect(controller.overwriteConflict).not.toHaveBeenCalled();
});
