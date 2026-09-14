// INPUT: Render failures and exact conversation identity changes.
// OUTPUT: Localized safe fallback; only a new identity clears the failed subtree.
// POS: Room error containment regression, no requests are replayed.
import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { RoomChatErrorBoundary } from "./room-chat-error-boundary";
it("contains render diagnostics and recovers when the conversation identity changes", () => {
  const report = vi.spyOn(console, "error").mockImplementation(() => undefined);
  function Broken(): never { throw new Error("private diagnostic"); }
  const view = (resetKey: string, broken: boolean) => <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: key => key }}>
    <RoomChatErrorBoundary resetKey={resetKey}>{broken ? <Broken /> : <p>Conversation</p>}</RoomChatErrorBoundary>
  </I18N_CONTEXT.Provider>;
  try {
    const { rerender } = render(view("room:a", true));
    expect(screen.getByText("room.chat_render_error_title")).toBeTruthy();
    expect(screen.queryByText("private diagnostic")).toBeNull();
    expect(screen.getByRole("button", { name: "common.refresh" })).toBeTruthy();
    rerender(view("room:a", false));
    expect(screen.queryByText("Conversation")).toBeNull();
    rerender(view("room:b", false));
    expect(screen.getByText("Conversation")).toBeTruthy();
    expect(report).toHaveBeenCalled();
  } finally { report.mockRestore(); }
});
