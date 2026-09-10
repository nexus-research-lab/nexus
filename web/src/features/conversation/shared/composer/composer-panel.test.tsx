// INPUT: An open WorkGraph picker followed by a Composer Session change.
// OUTPUT: Old picker intent cannot survive into the new Session draft.
// POS: Composer assembly lifecycle regression; child layout and controller behavior have their own suites.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ComposerPanel } from "./composer-panel";
import type { ComposerPanelProps } from "./composer-model";
const writeDraft = vi.hoisted(() => vi.fn());
vi.mock("./controller/use-composer-controller", () => ({ useComposerController: () => ({
  actions: { handleInputChange: writeDraft, setIsActionMenuOpen: vi.fn() }, attachments: {}, localDirectories: {}, mention: {}, refs: {}, sessionSettings: {}, slashCommand: {}, state: {},
}) }));
vi.mock("./use-composer-interaction-height-guard", () => ({ useComposerInteractionHeightGuard: vi.fn() }));
vi.mock("./components/footer/composer-footer", () => ({ ComposerFooter: ({ onWorkGraphDistillationsSelect }: { onWorkGraphDistillationsSelect: () => void }) => <button onClick={onWorkGraphDistillationsSelect}>WorkGraphs</button> }));
vi.mock("./components/workgraph-distillation-picker/workgraph-distillation-picker-dialog", () => ({ WorkGraphDistillationPickerDialog: ({ isOpen }: { isOpen: boolean }) => isOpen ? <div role="dialog">Picker</div> : null }));
vi.mock("./components/footer/composer-session-settings-reliability", () => ({ ComposerSessionSettingsReliability: () => null }));
vi.mock("./components/composer-input-row", () => ({ ComposerInputRow: () => null }));
vi.mock("./components/composer-local-directories", () => ({ ComposerLocalDirectories: () => null }));
vi.mock("./components/pending-queue/composer-pending-queue", () => ({ ComposerPendingQueue: () => null }));
vi.mock("./attachments/composer-local-attachments", () => ({ ComposerAttachmentList: () => null }));

describe("Composer picker scope", () => {
  it("closes the picker when switching Session and does not revive it on return", async () => {
    const user = userEvent.setup();
    const props = { draftScopeKey: "session-a", workGraphSessionKey: "session-a", inputQueueItems: [] } as unknown as ComposerPanelProps;
    const view = (value: ComposerPanelProps) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><ComposerPanel {...value} /></I18N_CONTEXT.Provider>;
    const rendered = render(view(props));
    await user.click(screen.getByRole("button", { name: "WorkGraphs" }));
    expect(screen.getByRole("dialog")).toBeTruthy();
    rendered.rerender(view({ ...props, draftScopeKey: "session-b", workGraphSessionKey: "session-b" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    rendered.rerender(view(props));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(writeDraft).not.toHaveBeenCalled();
  });
});
