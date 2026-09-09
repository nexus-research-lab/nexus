// INPUT: Team read-model snapshots and controlled send outcomes.
// OUTPUT: IME-safe submit, draft retention and accessible load/error feedback.
// POS: Team page interaction regressions; no Team transport is invoked.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { useTeamRoom } from "@/features/team/use-team-room";
import { TeamPage } from "./team-page";
const model = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock("@/features/team/use-team-room", () => ({ useTeamRoom: model.read }));
let room: ReturnType<typeof useTeamRoom>;
const originalScroll = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "scrollIntoView");
afterEach(() => {
  if (originalScroll) Object.defineProperty(HTMLElement.prototype, "scrollIntoView", originalScroll);
  else Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
});
beforeEach(() => {
  HTMLElement.prototype.scrollIntoView = vi.fn();
  room = {bootstrap: {
    team: {id: "team", deployment_id: "deployment", name: "Team"},
    room: {id: "room", team_id: "team", name: "General"},
    conversation: {id: "conversation", room_id: "room", type: "team", high_water_message_seq: 0, sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
  }, error: null, isLoading: false, isSending: false, messages: [], reload: vi.fn(), send: vi.fn().mockResolvedValue(true)};
  model.read.mockImplementation(() => room);
});
function page() {
  return <I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}><TeamPage /></I18N_CONTEXT.Provider>;
}
it("does not send on composition confirmation or Shift+Enter, then sends exact text on Enter", async () => {
  render(page());
  const input = screen.getByRole("textbox", {name: "team.message"});
  await userEvent.type(input, "中文草稿");
  fireEvent.keyDown(input, {key: "Enter", isComposing: true});
  fireEvent.keyDown(input, {key: "Enter", keyCode: 229});
  fireEvent.keyDown(input, {key: "Enter", shiftKey: true});
  expect(room.send).not.toHaveBeenCalled();
  await userEvent.keyboard("{Enter}");
  expect(room.send).toHaveBeenCalledExactlyOnceWith("中文草稿");
  expect((input as HTMLTextAreaElement).value).toBe("");
});
it("retains the draft after a rejected send and refuses empty or busy submissions", async () => {
  room.send = vi.fn().mockResolvedValue(false);
  const view = render(page());
  const input = screen.getByRole("textbox", {name: "team.message"});
  fireEvent.submit(input.closest("form")!);
  expect(room.send).not.toHaveBeenCalled();
  await userEvent.type(input, "Keep me");
  await userEvent.click(screen.getByRole("button", {name: "team.send"}));
  expect((input as HTMLTextAreaElement).value).toBe("Keep me");
  room = {...room, isSending: true};
  view.rerender(page());
  fireEvent.submit(input.closest("form")!);
  expect(room.send).toHaveBeenCalledTimes(1);
});
it("announces loading and load failure without claiming an empty conversation", () => {
  room = {...room, bootstrap: null, isLoading: true};
  const view = render(page());
  expect(screen.getByRole("status").textContent).toBe("team.loading");
  room = {...room, isLoading: false, error: "load"};
  view.rerender(page());
  expect(screen.getByRole("alert").textContent).toBe("team.error_load");
  expect(screen.queryByText("team.empty")).toBeNull();
});

it("keeps complete Unicode initials decorative while preserving author and Markdown content", () => {
  room.messages = [{id: "message", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "internal-user", author_username: "researcher",
    author_display_name: "👩‍🔬 Researcher", client_message_id: "client",
    content: {version: 1, blocks: [{type: "markdown", text: "**Research result**"}]},
    created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  expect(screen.getByText("👩‍🔬").getAttribute("aria-hidden")).toBe("true");
  expect(screen.getByText("👩‍🔬 Researcher")).toBeTruthy();
  expect(screen.getByText("Research result").tagName).toBe("STRONG");
  expect(screen.queryByText("internal-user")).toBeNull();
});
