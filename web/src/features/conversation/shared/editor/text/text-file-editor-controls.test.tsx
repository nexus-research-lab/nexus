// INPUT: Read/save facts, editor presentation and explicit user commands.
// OUTPUT: A single recovery surface preserves permitted actions and header availability.
// POS: Offline view behavior tests; controller revisions and file I/O retain their own tests.
import type { ComponentProps, ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { TextFileEditorHeader } from "./text-file-editor-header";
import { TextFileEditorReliability } from "./text-file-editor-reliability";
import type { TextFileSaveIssue } from "./text-file-editor-recovery";

vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: () => false }));
vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn(async () => undefined) }));

type ReliabilityProps = ComponentProps<typeof TextFileEditorReliability>;
function props(overrides: Partial<ReliabilityProps> = {}): ReliabilityProps {
  return {
    hasLoadedContent: true, isLoading: false, isReconciling: false, isSaving: false,
    onAdoptLatest: vi.fn(), onLoadLatest: vi.fn(), onOverwrite: vi.fn(), onReconcile: vi.fn(), onRetrySave: vi.fn(),
    resourceFailure: null, revisionReady: true, saveIssue: null, ...overrides,
  };
}
function localized(children: ReactNode, locale: Locale = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>{children}</I18N_CONTEXT.Provider>;
}
function expectNoCommands(state: ReliabilityProps) {
  for (const command of [state.onAdoptLatest, state.onLoadLatest, state.onOverwrite, state.onReconcile, state.onRetrySave]) {
    expect(command).not.toHaveBeenCalled();
  }
}
const unknownIssue: TextFileSaveIssue = { kind: "outcome_unknown", attemptedDraft: "draft", expectedRevision: "revision", reconciliationFailed: false };

it("shows no recovery surface and issues no commands without a failure", () => {
  const state = props();
  const { container } = render(localized(<TextFileEditorReliability {...state} />));
  expect(container.textContent).toBe("");
  expectNoCommands(state);
});

it.each([
  { access: null, loaded: false, impact: "workspace_file.load_failed_empty_impact" },
  { access: null, loaded: true, impact: "workspace_file.load_failed_stale_impact" },
  { access: "forbidden", loaded: true, impact: "workspace_file.access_failure_impact" },
] as const)("prioritizes read failures with access=$access and loaded=$loaded", ({ access, loaded, impact }) => {
  const state = props({ hasLoadedContent: loaded, resourceFailure: { access, message: "raw diagnostic" }, saveIssue: unknownIssue });
  render(localized(<TextFileEditorReliability {...state} />));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText(MESSAGES.en[access ? "state.permission_title" : "workspace_file.load_failed_title"])).toBeTruthy();
  expect(screen.getByText(MESSAGES.en[impact])).toBeTruthy();
  expect(screen.queryByText("raw diagnostic")).toBeNull();
  expect(screen.getAllByRole("button")).toHaveLength(1);
  expectNoCommands(state);
  fireEvent.click(screen.getByRole("button", { name: MESSAGES.en["state.retry"] }));
  expect(state.onLoadLatest).toHaveBeenCalledTimes(1);
  expect(state.onRetrySave).not.toHaveBeenCalled();
  expect(state.onReconcile).not.toHaveBeenCalled();
});

const recoveryCases = [
  { issue: { kind: "conflict", phase: "reload_required" }, title: "workspace_file.conflict_title", action: "workspace_file.load_latest", command: "onLoadLatest" },
  { issue: unknownIssue, title: "workspace_file.save_unknown_title", action: "workspace_file.check_save_result", command: "onReconcile" },
  { issue: { ...unknownIssue, reconciliationFailed: true }, title: "workspace_file.save_check_failed_title", action: "workspace_file.check_save_result", command: "onReconcile" },
  { issue: { kind: "retry_ready" }, title: "workspace_file.save_retry_ready_title", action: "workspace_file.retry_save", command: "onRetrySave" },
  { issue: { kind: "not_applied", detail: "raw save detail" }, title: "workspace_file.save_not_applied_title", action: "workspace_file.retry_save", command: "onRetrySave" },
] as const;
it.each(recoveryCases)("projects $title with only its explicit recovery action", ({ issue, title, action, command }) => {
  const state = props({ saveIssue: issue });
  const view = render(localized(<TextFileEditorReliability {...state} />));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText(MESSAGES.en[title])).toBeTruthy();
  expect(screen.queryByText("raw save detail")).toBeNull();
  expectNoCommands(state);
  view.rerender(localized(<TextFileEditorReliability {...state} />, "zh"));
  expect(screen.getByText(MESSAGES.zh[title])).toBeTruthy();
  expect(screen.getAllByRole("button")).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: MESSAGES.zh[action] }));
  expect(state[command]).toHaveBeenCalledTimes(1);
  const commandCalls = [state.onLoadLatest, state.onReconcile, state.onRetrySave].map((callback) => vi.mocked(callback).mock.calls.length);
  expect(commandCalls.reduce((sum, count) => sum + count, 0)).toBe(1);
  expect(state.onOverwrite).not.toHaveBeenCalled();
});

it("requires a current revision to overwrite reviewed content and keeps adoption explicit", () => {
  const state = props({ saveIssue: { kind: "conflict", phase: "review" }, revisionReady: false });
  const view = render(localized(<TextFileEditorReliability {...state} />));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getAllByRole("button")).toHaveLength(2);
  expectNoCommands(state);
  const overwrite = () => screen.getByRole("button", { name: MESSAGES.en["workspace_file.overwrite_latest"] });
  expect((overwrite() as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(overwrite());
  expect(state.onOverwrite).not.toHaveBeenCalled();
  view.rerender(localized(<TextFileEditorReliability {...state} revisionReady />));
  fireEvent.click(overwrite());
  expect(state.onOverwrite).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: MESSAGES.en["workspace_file.adopt_latest"] }));
  expect(state.onAdoptLatest).toHaveBeenCalledTimes(1);
  expect(state.onRetrySave).not.toHaveBeenCalled();
});

it.each([
  { saveIssue: { kind: "conflict", phase: "reload_required" }, busy: "isLoading", label: "workspace_file.loading_latest", command: "onLoadLatest" },
  { saveIssue: unknownIssue, busy: "isReconciling", label: "workspace_file.checking_save_result", command: "onReconcile" },
  { saveIssue: { kind: "retry_ready" }, busy: "isSaving", label: "common.saving", command: "onRetrySave" },
  { saveIssue: { kind: "conflict", phase: "review" }, busy: "isSaving", label: "workspace_file.overwriting", command: "onOverwrite" },
] as const)("prevents repeating $command while $busy", ({ saveIssue, busy, label, command }) => {
  const state = props({ saveIssue, [busy]: true });
  render(localized(<TextFileEditorReliability {...state} />));
  const action = screen.getByRole("button", { name: MESSAGES.en[label] });
  expect((action as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(action);
  expect(state[command]).not.toHaveBeenCalled();
});

it("preserves header commands, disabled states and one lightweight sync label", () => {
  const state: ComponentProps<typeof TextFileEditorHeader> = {
    agentId: "agent", path: "report.md", fileName: "report.md", isPreviewFocused: false,
    onSave: vi.fn(), onToggleEditing: vi.fn(), onTogglePreviewFocus: vi.fn(),
    presentation: { bodyMode: "preview", editAction: "edit", editDisabled: true, editLabel: "Edit", saveDisabled: true, saveLabel: "Save", sync: { kind: "writing", label: "Writing" } },
  };
  const view = render(localized(<TextFileEditorHeader {...state} />));
  expect(screen.getAllByText("Writing")).toHaveLength(1);
  for (const name of ["Edit", "Save"]) {
    expect((screen.getByRole("button", { name }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name }));
  }
  expect(state.onSave).not.toHaveBeenCalled();
  expect(state.onToggleEditing).not.toHaveBeenCalled();
  view.rerender(localized(<TextFileEditorHeader {...state} presentation={{ ...state.presentation, editDisabled: false, saveDisabled: false, sync: { kind: "synced", label: "Synced" } }} />));
  expect(screen.queryByText("Writing")).toBeNull();
  expect(screen.getAllByText("Synced")).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "Edit" }));
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(state.onToggleEditing).toHaveBeenCalledTimes(1);
  expect(state.onSave).toHaveBeenCalledTimes(1);
});
