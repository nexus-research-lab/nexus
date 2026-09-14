// INPUT: Exact Agent profile editor, confirmation, deferred save and newer draft state.
// OUTPUT: Named source editing, synchronous submit lock and isolation of late completion.
// POS: Profile UI regression; revision, access and save results come from the existing file controller.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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
async function confirmButton() {
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  return within(screen.getByRole("dialog")).getByRole("button", { name: MESSAGES.en["agent_options.identity.profile_save_confirm_action"] });
}

it("binds the visible profile label to the real source editor and keeps blur in edit mode", async () => {
  render(view());
  const source = screen.getByRole("textbox", { name: "Behavior template" });
  expect(document.activeElement).toBe(source);
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(first.setIsEditing).not.toHaveBeenCalled();
  expect(first.save).not.toHaveBeenCalled();
});

it("locks one save before the controller renders busy and reveals failure without exiting editing", async () => {
  let finish!: (saved: boolean) => void;
  first.save = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  render(view()); const confirm = await confirmButton();
  act(() => { fireEvent.click(confirm); fireEvent.click(confirm); });
  expect(first.save).toHaveBeenCalledOnce();
  const dialog = screen.getByRole("dialog");
  expect(within(dialog).getAllByRole("button").every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
  await userEvent.keyboard("{Escape}"); expect(screen.getByRole("dialog")).toBe(dialog);
  await act(async () => finish(false));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(first.setIsEditing).not.toHaveBeenCalled();
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("# Agent profile");
});

it("does not let an old Agent save close the new Agent's confirmation", async () => {
  let finish!: (saved: boolean) => void;
  first.save = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view()); await userEvent.click(await confirmButton());
  rerender(view("second")); expect(screen.queryByRole("dialog")).toBeNull();
  await confirmButton(); const current = screen.getByRole("dialog");
  await act(async () => finish(true));
  expect(screen.getByRole("dialog")).toBe(current);
  expect(second.save).not.toHaveBeenCalled(); expect(second.setIsEditing).not.toHaveBeenCalled();
});

it("retains editing when a newer draft arrives during save", async () => {
  let finish!: (saved: boolean) => void;
  first.save = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view()); await userEvent.click(await confirmButton());
  first = { ...first, displayContent: "newer draft", draftContent: "newer draft" };
  rerender(view()); await act(async () => finish(true));
  expect(first.setIsEditing).not.toHaveBeenCalled();
  expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("newer draft");
});

it("exits editing after the same draft is saved successfully", async () => {
  render(view()); await userEvent.click(await confirmButton());
  expect(first.setIsEditing).toHaveBeenCalledExactlyOnceWith(false);
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("closes a superseded confirmation to expose a conflict without saving", async () => {
  const { rerender } = render(view()); await confirmButton();
  first = { ...first, saveIssue: { kind: "conflict", phase: "reload_required" } };
  rerender(view());
  expect(screen.queryByRole("dialog")).toBeNull(); expect(first.save).not.toHaveBeenCalled();
  expect(screen.getByText(MESSAGES.en["workspace_file.conflict_title"])).toBeTruthy();
});

it("does not offer a save confirmation without the required read revision", () => {
  first = { ...first, revision: null };
  render(view());
  expect((screen.getByRole("button", { name: "Save" }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(first.save).not.toHaveBeenCalled();
});
