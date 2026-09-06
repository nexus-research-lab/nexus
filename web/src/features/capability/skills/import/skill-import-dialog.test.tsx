// INPUT: Real import dialog/hook, controlled busy state and native local-file entry.
// OUTPUT: Exact Git submission, mode-preserved drafts, close-reset and independent busy locks.
// POS: Import composition regression; archive parsing and remote writes belong to marketplace controllers.

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";
import { SkillImportDialog } from "./skill-import-dialog";

function Harness({ importing = false, onImportGit, onClose, onFileOpen }: {
  importing?: boolean; onImportGit: ReturnType<typeof vi.fn>; onClose: ReturnType<typeof vi.fn>; onFileOpen: ReturnType<typeof vi.fn>;
}) {
  const [mode, setMode] = useState<SkillImportDialogMode | null>("git");
  const fileInputRef = useRef<HTMLInputElement>(null);
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <button onClick={() => setMode("git")} type="button">Open import</button>
    <input hidden onClick={onFileOpen} ref={fileInputRef} type="file" />
    <SkillImportDialog fileInputRef={fileInputRef} importing={importing} mode={mode}
      onClose={() => { onClose(); setMode(null); }} onImportGit={onImportGit} onSelectMode={setMode} />
  </I18N_CONTEXT.Provider>;
}

describe("SkillImportDialog", () => {
  it("preserves Git drafts through local-file selection, submits them exactly and resets only on close", async () => {
    const user = userEvent.setup();
    const commands = { onImportGit: vi.fn(), onClose: vi.fn(), onFileOpen: vi.fn() };
    render(<Harness {...commands} />);
    const field = (key: string) => screen.getByRole("textbox", { name: `capability.skills_import_git_${key}` });
    fireEvent.change(field("url"), { target: { value: "https://example.test/team/repo.git" } });
    fireEvent.change(field("branch"), { target: { value: " feature/skill " } });
    fireEvent.change(field("path"), { target: { value: "  skills/room playbook  " } });
    await user.click(screen.getByRole("button", { name: "capability.skills_import_mode_local" }));
    await user.click(screen.getByRole("button", { name: "capability.skills_import_choose_zip" }));
    expect(commands.onFileOpen).toHaveBeenCalledTimes(1);
    expect(commands.onImportGit).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "capability.skills_import_mode_git" }));
    expect((field("url") as HTMLInputElement).value).toBe("https://example.test/team/repo.git");
    expect((field("branch") as HTMLInputElement).value).toBe(" feature/skill ");
    expect((field("path") as HTMLInputElement).value).toBe("  skills/room playbook  ");
    await user.click(screen.getByRole("button", { name: "capability.skills_import_git_submit" }));
    expect(commands.onImportGit).toHaveBeenCalledExactlyOnceWith("https://example.test/team/repo.git", " feature/skill ", "  skills/room playbook  ");
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(commands.onClose).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Open import" }));
    expect((field("url") as HTMLInputElement).value).toBe("");
  });

  it("exposes import busy state and blocks mode changes, submission and dismissal", async () => {
    const user = userEvent.setup();
    const commands = { onImportGit: vi.fn(), onClose: vi.fn(), onFileOpen: vi.fn() };
    render(<Harness {...commands} importing />);
    const submit = screen.getByRole("button", { name: "capability.skills_importing" });
    expect(submit.getAttribute("aria-busy")).toBe("true");
    for (const input of screen.getAllByRole("textbox")) expect((input as HTMLInputElement).disabled).toBe(true);
    await user.click(screen.getByRole("button", { name: "capability.skills_import_mode_local" }));
    await user.click(submit);
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    await user.keyboard("{Escape}");
    expect(screen.getByRole("dialog", { name: "capability.skills_import_title" })).toBeTruthy();
    expect(commands.onImportGit).not.toHaveBeenCalled();
    expect(commands.onClose).not.toHaveBeenCalled();
    expect(commands.onFileOpen).not.toHaveBeenCalled();
  });
});
