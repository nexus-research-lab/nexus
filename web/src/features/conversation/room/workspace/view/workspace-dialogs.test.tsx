// INPUT: Real Workspace dialog adapter with local create/rename commands and execution state.
// OUTPUT: Every Prompt mode preserves exact text; an in-flight workspace mutation locks editing, resubmission and exit.
// POS: Workspace view regression without API calls or filesystem mutation.

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { WorkspacePromptState } from "../controller/interaction/workspace-interaction-model";
import { WorkspaceDialogs } from "./workspace-dialogs";

const prompts: [string, WorkspacePromptState][] = [
  ["Create file", { mode: "create-file", defaultValue: "untitled.txt", parentPath: null }],
  ["Create folder", { mode: "create-directory", defaultValue: "new-folder", parentPath: "docs" }],
  ["Rename", { mode: "rename", defaultValue: "old.txt", entry: { name: "old.txt", path: "docs/old.txt", is_dir: false, modified_at: "", depth: 1 } }],
];

describe("Workspace dialogs", () => {
  it.each(prompts)("uses the shared %s input and respects workspace busy state", async (title, promptState) => {
    const user = userEvent.setup();
    const noop = vi.fn();
    const asyncNoop = vi.fn(async () => undefined);
    const controller: ComponentProps<typeof WorkspaceDialogs>["controller"] = {
      closeContextMenu: noop, closeDeletePrompt: noop, closePrompt: noop,
      contextMenu: { entry: null, position: null }, deleteTarget: null, isMutating: false, promptState,
      handleUploadClick: noop, openCreatePrompt: noop, openRenamePrompt: noop,
      handlePromptConfirm: asyncNoop, handleConfirmDelete: asyncNoop, handleAddContextEntryToChat: asyncNoop,
      handleCopyContextEntryPath: asyncNoop, handleDownloadContextEntry: asyncNoop, handleOpenContextEntry: asyncNoop,
      openApplications: null, openDeletePrompt: noop,
    };
    const view = (busy: boolean) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: noop, t: (key) => MESSAGES.en[key] }}>
      <WorkspaceDialogs controller={{ ...controller, isMutating: busy }} />
    </I18N_CONTEXT.Provider>;
    const { rerender } = render(view(false));
    const input = screen.getByRole("textbox", { name: title }) as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "  新名称.txt  ");
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    expect(asyncNoop).not.toHaveBeenCalled();
    rerender(view(true));
    expect(input.disabled).toBe(true);
    await user.click(screen.getByRole("button", { name: "Confirm" }));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(asyncNoop).not.toHaveBeenCalled();
    expect(noop).not.toHaveBeenCalled();
    rerender(view(false));
    await user.click(screen.getByRole("button", { name: "Confirm" }));
    expect(asyncNoop).toHaveBeenCalledExactlyOnceWith("  新名称.txt  ");
  });
});
