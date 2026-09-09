// INPUT: Team read-model snapshots and controlled send outcomes.
// OUTPUT: IME-safe submit, draft retention and accessible load/error feedback.
// POS: Team page interaction regressions; no Team transport is invoked.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { useTeamRoom } from "@/features/team/use-team-room";
import { MemoryRouter } from "react-router-dom";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";
import { TeamPage } from "./team-page";
const model = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock("@/features/team/use-team-room", () => ({ useTeamRoom: model.read }));
let room: ReturnType<typeof useTeamRoom>;
beforeEach(() => {
  room = {room: {
    room: {id: "room", team_id: "team", name: "General", description: "", avatar: "", configuration_version: 1, membership_version: 1, created_at: "2026-09-09T00:00:00Z", updated_at: "2026-09-09T00:00:00Z"},
    conversation: {id: "conversation", room_id: "room", type: "main", high_water_message_seq: 0, last_activity_at: null, sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
    current_user_role: "owner",
  }, error: null, isLoading: false, isSending: false, messages: [], reload: vi.fn(), retryLoad: vi.fn(), send: vi.fn().mockResolvedValue(true)};
  model.read.mockImplementation(() => room);
});
function page() {
  return <I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}><AUTH_CONTEXT.Provider value={{error: null, isBootstrapped: true, loading: false, login: vi.fn(), logout: vi.fn(), refreshStatus: vi.fn(), status: {auth_required: true, authenticated: true, auth_method: "password", password_login_enabled: true, user_id: "owner", username: "owner"}}}><MemoryRouter initialEntries={["/team?room_id=room"]}><TeamPage /></MemoryRouter></AUTH_CONTEXT.Provider></I18N_CONTEXT.Provider>;
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
  room = {...room, room: null, isLoading: true};
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

it("keeps a reader in place as messages arrive, then resumes following through the shared action", async () => {
  const view = render(page());
  const viewport = screen.getByRole("region", {name: "team.shared_room"});
  let height = 600;
  Object.defineProperties(viewport, {
    clientHeight: {configurable: true, value: 200},
    scrollHeight: {configurable: true, get: () => height},
  });
  viewport.scrollTop = 400;
  fireEvent.scroll(viewport);
  fireEvent.wheel(viewport, {deltaY: -100});
  viewport.scrollTop = 150;
  fireEvent.scroll(viewport);
  expect(screen.getByRole("button", {name: "room.scroll_to_latest"})).toBeTruthy();
  room.messages = [{id: "new", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "user", author_username: "name", author_display_name: "Name",
    client_message_id: "client", content: {version: 1, blocks: [{type: "markdown", text: "New message"}]}, created_at: "2026-09-09T01:00:00Z"}];
  height = 800;
  view.rerender(page());
  expect(viewport.scrollTop).toBe(150);
  await userEvent.click(screen.getByRole("button", {name: "room.scroll_to_latest"}));
  await waitFor(() => expect(viewport.scrollTop).toBe(600));
  expect(screen.queryByRole("button", {name: "room.scroll_to_latest"})).toBeNull();
  room.messages = [...room.messages, {...room.messages[0], id: "next", message_seq: 2}];
  height = 1_000;
  view.rerender(page());
  await waitFor(() => expect(viewport.scrollTop).toBe(800));
});

it("offers load retry without sending and disables it while loading", async () => {
  room = {...room, error: "load", room: null};
  const view = render(page());
  await userEvent.click(screen.getByRole("button", {name: "state.retry"}));
  expect(room.retryLoad).toHaveBeenCalledOnce();
  expect(room.send).not.toHaveBeenCalled();
  room = {...room, isLoading: true};
  view.rerender(page());
  expect((screen.getByRole("button", {name: "state.retry"}) as HTMLButtonElement).disabled).toBe(true);
});

it("uses shared Room surfaces and resolves the requested online room", async () => {
  const {container} = render(page());
  expect(model.read).toHaveBeenCalledWith("room");
  expect(container.querySelector(".workspace-surface-header")).toBeTruthy();
  expect(container.querySelector(".nexus-chat-composer-shell")).toBeTruthy();
  const input = screen.getByRole("textbox", {name: "team.message"});
  fireEvent.change(input, {target: {value: "hello"}});
  fireEvent.click(screen.getByRole("button", {name: "team.send"}));
  await waitFor(() => expect(room.send).toHaveBeenCalledExactlyOnceWith("hello"));
});
