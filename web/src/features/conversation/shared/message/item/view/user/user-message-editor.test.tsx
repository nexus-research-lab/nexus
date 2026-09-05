// INPUT: Real message view/controller, edit drafts and native keyboard commands.
// OUTPUT: Exact-round submission, empty/unchanged suppression and cancellation regression evidence.
// POS: User-message editing integration; all commands terminate in the local Gallery fixture.

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MessageSurfacesGallery } from "@/dev/ui-gallery/ui-gallery-message-surfaces";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { MessageUserSection } from "./message-user-section";
import type { UserMessage } from "@/types/conversation/message/entity";

describe("User message editing", () => {
  it("withholds edit/rerun only for durable Goal control records, not ordinary slash text", () => {
    const onEdit = vi.fn();
    const message: UserMessage = {
      message_id: "goal-control", session_key: "goal-session", agent_id: "author", round_id: "goal-round", role: "user",
      timestamp: 1788566400000, content: "/goal Preserve exact file scope", metadata: { subtype: "goal_set" },
    };
    const view = (value: UserMessage) => <I18nProvider><MessageUserSection compact message={value} onEditUserMessage={onEdit} /></I18nProvider>;
    const { rerender, container } = render(view(message));
    expect(screen.queryByRole("button", { name: /Edit message|编辑消息/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Run again|重新运行/ })).toBeNull();
    expect(container.querySelector('[data-goal-control="true"]')?.textContent).toContain("Preserve exact file scope");
    expect(container.querySelector('[data-goal-control="true"]')?.textContent).not.toContain("/goal");
    rerender(view({ ...message, metadata: undefined }));
    expect(screen.getByRole("button", { name: /Edit message|编辑消息/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Run again|重新运行/ })).toBeTruthy();
    expect(container.querySelector('[data-goal-control="false"]')?.textContent).toContain("/goal");
    expect(onEdit).not.toHaveBeenCalled();
  });

  it("submits only a changed nonempty draft with the original round identity", () => {
    const { container } = render(<I18nProvider><MessageSurfacesGallery /></I18nProvider>);
    const view = container.querySelector<HTMLElement>("[data-gallery-message-editor]")!;
    const output = container.querySelector("[data-gallery-message-commands]")!;
    const edit = () => fireEvent.click(within(view).getByRole("button", { name: /edit message|编辑消息/i }));
    edit();
    const input = within(view).getByRole("textbox");
    expect(document.activeElement).toBe(input);
    fireEvent.change(input, { target: { value: "  Revised message.  " } });
    for (const key of [
      { key: "Enter", ctrlKey: true, isComposing: true },
      { key: "Enter", metaKey: true, keyCode: 229 },
      { key: "Escape", isComposing: true },
    ]) {
      fireEvent.keyDown(input, key);
      expect(within(view).getByRole("textbox")).toBe(input);
      expect(JSON.parse(output.textContent!)).toEqual([]);
    }
    fireEvent.keyDown(input, { key: "Enter", ctrlKey: true });
    expect(JSON.parse(output.textContent!)).toEqual([{ round: "gallery-round", content: "Revised message." }]);
    expect(within(view).queryByRole("textbox")).toBeNull();

    for (const value of ["Revised message.", "   "]) {
      edit();
      fireEvent.change(within(view).getByRole("textbox"), { target: { value } });
      expect(within(view).getByRole("button", { name: /send|发送/i }).hasAttribute("disabled")).toBe(true);
      fireEvent.keyDown(within(view).getByRole("textbox"), { key: "Enter", metaKey: true });
      expect(JSON.parse(output.textContent!)).toHaveLength(1);
      expect(within(view).queryByRole("textbox")).toBeNull();
    }
    edit();
    fireEvent.change(within(view).getByRole("textbox"), { target: { value: "Discard this edit" } });
    fireEvent.keyDown(within(view).getByRole("textbox"), { key: "Escape" });
    expect(JSON.parse(output.textContent!)).toHaveLength(1);
    expect(within(view).getByText("Revised message.")).toBeTruthy();
    expect(screen.queryByDisplayValue("Discard this edit")).toBeNull();
  });
});
