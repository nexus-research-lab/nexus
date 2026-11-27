// INPUT: Exact Agent profile editor, direct save, deferred save and newer draft state.
// OUTPUT: Named source editing, synchronous submit lock and isolation of late completion.
// POS: Profile UI regression; revision, access and save results come from the existing file controller.

import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useTextFileEditor } from "@/features/conversation/shared/editor/text/use-text-file-editor";
import { AgentProfileFileEditor } from "./agent-profile-file-editor";

vi.mock("@/features/conversation/shared/editor/text/use-text-file-editor", () => ({ useTextFileEditor: vi.fn() }));
vi.mock("@/features/conversation/shared/editor/text/text-file-content", () => ({ TextFileContent: () => <div>Preview boundary</div> }));

type Editor = ReturnType<typeof useTextFileEditor>;
function makeEditor(): Editor {
  return {
    adoptLatest: vi.fn(), displayContent: "# Agent profile", draftContent: "# Agent profile", hasLoadedContent: true,
    isDirty: true, isEditing: true, isExternalWriting: false, isLoading: false, isReconciling: false, isSaving: false,
    liveState: undefined, loadContent: vi.fn(async () => undefined), overwriteConflict: vi.fn(async () => false),
    reconcileSave: vi.fn(async () => undefined), requiresChunkedPreview: false, resourceFailure: null, revision: "revision-1",
    save: vi.fn(async () => true), saveIssue: null, setDraftContent: vi.fn(), setIsEditing: vi.fn(), toggleEditing: vi.fn(),
  };
}
let first: Editor;
let second: Editor;
beforeEach(() => {
  first = makeEditor(); second = makeEditor();
  vi.mocked(useTextFileEditor).mockImplementation(({ agentId }) => agentId === "first" ? first : second);
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
});
afterEach(() => vi.unstubAllGlobals());
function view(agentId = "first") {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}>
    <AgentProfileFileEditor agentId={agentId} label="Behavior template" />
  </I18N_CONTEXT.Provider>;
}
function saveButton() {
  return screen.getByRole("button", { name: "Save" });
}

it("binds the visible profile label to the real source editor", () => {
  render(view());
  const source = screen.getByRole("textbox", { name: "Behavior template" });
  expect(document.activeElement).toBe(source);
});

it("saves directly without a confirmation dialog and exits after the same draft is saved", async () => {
  render(view()); await userEvent.click(saveButton());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(first.save).toHaveBeenCalledOnce();
  expect(first.setIsEditing).toHaveBeenCalledExactlyOnceWith(false);
});

it("does not let an old Agent save close the new Agent's editor", async () => {
  let finish!: (saved: boolean) => void;
  first.save = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view()); await userEvent.click(saveButton());
  rerender(view("second"));
  await act(async () => finish(true));
  expect(second.save).not.toHaveBeenCalled(); expect(second.setIsEditing).not.toHaveBeenCalled();
});

it("retains editing when a newer draft arrives during save", async () => {
  let finish!: (saved: boolean) => void;
  first.save = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view()); await userEvent.click(saveButton());
  first = { ...first, displayContent: "newer draft", draftContent: "newer draft" };
  rerender(view()); await act(async () => finish(true));
  expect(first.setIsEditing).not.toHaveBeenCalled();
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("newer draft");
});

it("blocks saving while a conflict needs resolution", () => {
  first = { ...first, saveIssue: { kind: "conflict", phase: "reload_required" } };
  render(view());
  expect((saveButton() as HTMLButtonElement).disabled).toBe(true);
  expect(first.save).not.toHaveBeenCalled();
  expect(screen.getByText(MESSAGES.en["workspace_file.conflict_title"])).toBeTruthy();
});
