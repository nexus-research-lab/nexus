// INPUT: Team read-model snapshots and controlled send outcomes.
// OUTPUT: IME-safe submit, draft retention and accessible load/error feedback.
// POS: Team page interaction regressions; no Team transport is invoked.
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { useTeamRoom } from "@/features/team/use-team-room";
import { MemoryRouter, useNavigate } from "react-router-dom";
import { AUTH_CONTEXT } from "@/shared/auth/auth-context";
import { TeamPage } from "./team-page";
const model = vi.hoisted(() => ({ agents: vi.fn(), read: vi.fn(), node: vi.fn(), prepare: vi.fn(), observe: vi.fn(), upload: vi.fn(), commands: vi.fn() }));
vi.mock("@/lib/api/conversation/team-files-api", () => ({uploadTeamFile: model.upload, saveTeamFile: vi.fn()}));
vi.mock("@/lib/api/conversation/team-api", async (original) => ({
  ...await original<typeof import("@/lib/api/conversation/team-api")>(), getTeamCommands: model.commands,
}));
vi.mock("@/features/team/team-execution-thread", async (original) => ({
  ...await original<typeof import("@/features/team/team-execution-thread")>(),
  TeamExecutionObserver: (props: unknown) => { model.observe(props); return null; },
}));
vi.mock("@/lib/api/conversation/team-node-api", () => ({ getTeamNode: model.node, prepareTeamRoom: model.prepare }));
vi.mock("@/features/team/use-team-room", () => ({ useTeamRoom: model.read }));
vi.mock("@/features/team/use-team-members", () => ({ useTeamMembers: () => [] }));
vi.mock("@/features/team/team-room-members-dialog", () => ({ TeamRoomMembersDialog: () => null }));
vi.mock("@/features/team/team-workspace", () => ({ TeamWorkspace: () => <div>shared files</div> }));
vi.mock("@/features/home/home-directory-resource", () => ({ useHomeDirectory: () => ({ agents: [] }) }));
vi.mock("@/lib/api/account/control-api", () => ({ listControlAgentDirectoryApi: model.agents }));
let room: ReturnType<typeof useTeamRoom>;
beforeEach(() => {
  model.upload.mockReset().mockRejectedValue(new Error("upload failed"));
  model.commands.mockReset().mockResolvedValue({status:"ready", commands:[{name:"browser",description:"Browser",execution:"runtime",enabled:true}]});
  model.observe.mockClear();
  model.prepare.mockReset().mockResolvedValue([]);
  model.node.mockReset().mockResolvedValue({state: "disconnected", jobs: []});
  model.agents.mockClear();
  Object.defineProperty(HTMLElement.prototype, "scrollTo", { configurable: true, value: vi.fn() });
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", { configurable: true, value: vi.fn() });
  model.agents.mockImplementation(() => new Promise(() => undefined));
  room = {pendingText: null, pendingAttachmentNames: [], room: {
    room: {id: "room", organization_id: "organization", team_id: "team", name: "General", description: "", avatar: "", host_auto_reply_enabled: false, private_messages_enabled: false, skill_names: [], configuration_version: 1, membership_version: 1, created_at: "2026-09-09T00:00:00Z", updated_at: "2026-09-09T00:00:00Z"},
    conversation: {id: "conversation", room_id: "room", type: "main", high_water_message_seq: 0, last_activity_at: null, sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
    current_user_role: "owner",
	members: [],
  }, error: null, isLoading: false, isSending: false, hasUnconfirmedSend: false, updateDetails: vi.fn(), messages: [], retryLoad: vi.fn(), send: vi.fn().mockResolvedValue(true)};
  model.read.mockImplementation(() => room);
});
function SwitchRoom() {
  const navigate = useNavigate();
  return <button onClick={() => navigate("/team?room_id=other")}>Switch room</button>;
}
function page(organizationId = "organization", avatar?: string) {
  return <I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: (key) => key}}><AUTH_CONTEXT.Provider value={{error: null, isBootstrapped: true, loading: false, login: vi.fn(), logout: vi.fn(), refreshStatus: vi.fn(), status: {organization_id: organizationId, auth_required: true, authenticated: true, auth_method: "password", password_login_enabled: true, user_id: "local-owner", control_user_id: "owner", username: "owner", avatar}}}><MemoryRouter initialEntries={["/team?room_id=room"]}><TeamPage /><SwitchRoom /></MemoryRouter></AUTH_CONTEXT.Provider></I18N_CONTEXT.Provider>;
}
it("keeps the room avatar independent from members and resolves my remote avatar", () => {
  room.room!.room.avatar = "12";
  room.room!.members = [{room_id: "room", member_type: "user", member_id: "owner", role: "owner", state: "active", invited_by_user_id: "owner", joined_at: "", created_at: "", updated_at: ""}];
  const {container} = render(page("organization", "/icon/agent/3.png"));
  expect(container.querySelector('.workspace-surface-header-identity-avatar img')?.getAttribute("src")).toBe("/icon/room/12.png");
  expect(container.querySelector('.workspace-surface-header-member-avatars img')?.getAttribute("src")).toBe("/icon/agent/3.png");
});
it("opens the shared header panels without inventing a remote Agent execution session", () => {
  render(page());
  for (const name of ["room.workgraph", "subagents.label", "room.workspace", "room.about"]) {
    fireEvent.click(screen.getByRole("button", {name}));
    expect(screen.getByText(name === "room.workspace" ? "shared files" : name === "room.about" ? "team.local_execution_empty" : "team.execution_empty")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", {name: "common.close"}));
    expect(screen.queryByText("team.local_execution_empty")).toBeNull();
  }
});
it("uses the remote Control identity for the own-message surface on desktop", () => {
  room.messages = [{id: "mine", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "owner", author_username: "owner", author_display_name: "My Remote Name",
    client_message_id: "mine", content: {version: 1, blocks: [{type: "markdown", text: "My message"}]}, created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  expect(screen.getByText("My message")).toBeTruthy();
  expect(screen.queryByText("My Remote Name")).toBeNull();
});

it("keeps organization-less remote accounts outside the online conversation", () => {
  model.agents.mockClear();
  render(page(""));
  expect(screen.queryByRole("textbox",{name:"team.message"})).toBeNull();
  expect(model.agents).not.toHaveBeenCalled();
});
it("renders my Agent as an independent member, without a human own-message bubble", async () => {
  model.agents.mockResolvedValue([{agent_id: "agent", name: "Research Agent"}]);
  room.messages = [{id: "result", conversation_id: "conversation", message_seq: 1,
    author_type: "agent", author_agent_id: "agent", author_user_id: "owner", author_username: "agent", author_display_name: "agent",
    delivery_id: "delivery", output_kind: "final", client_message_id: "output", content: {version: 1, blocks: [{type: "markdown", text: "**Completed**"}]}, created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  expect(await screen.findByText("Research Agent")).toBeTruthy();
  expect(screen.queryByText("Agent")).toBeNull();
  expect(screen.getByText("Completed").closest(".nexus-chat-message-round")).toBeTruthy();
  expect(screen.getByText("Completed").tagName).toBe("STRONG");
});
it("renders online execution statistics through the shared Room footer", async () => {
  model.agents.mockResolvedValue([{agent_id: "agent", name: "Lucy"}]);
  room.messages = [{id: "stats", conversation_id: "conversation", message_seq: 1,
    author_type: "agent", author_agent_id: "agent", author_user_id: "owner", author_username: "agent", author_display_name: "Lucy",
    client_message_id: "output", content: {version: 1, blocks: [{type: "markdown", text: "完成"}],
      execution: {model: "glm-5.3-flash", result_summary: {duration_ms: 53400, duration_api_ms: 50000, num_turns: 1, total_cost_usd: 0.0785, usage: {input_tokens: 3500, output_tokens: 1600, cache_read_input_tokens: 40000}}}},
    created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  expect(await screen.findByText("53.4s")).toBeTruthy();
  expect(screen.getByText("3.5k↑ 1.6k↓")).toBeTruthy();
  expect(screen.getByText("$0.0785")).toBeTruthy();
  expect(screen.getByText("glm-5.3-flash")).toBeTruthy();
});
it("shows remote delivery progress and failure without exposing a private Thread", async () => {
  model.agents.mockResolvedValue([{agent_id: "agent", name: "Amy"}]);
  const trigger: import("@/lib/api/conversation/team-api").TeamMessage = {id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user", author_user_id: "other", author_username: "Other", author_display_name: "Other", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "Request"}]}, created_at: "2026-09-09T01:00:00Z"};
  room.messages = [trigger];
  const delivery = {id: "delivery", message_id: "trigger", agent_id: "agent", state: "pending" as const};
  room.room!.deliveries = [delivery];
  const view = render(page());
  expect(await screen.findByText("team.delivery_pending")).toBeTruthy();
  expect(screen.queryByRole("button", {name: "room.thread_action_open"})).toBeNull();
  room.room!.deliveries = [{...delivery, state: "leased"}];
  room.messages.push({...trigger, id: "intermediate", author_type: "agent", author_agent_id: "agent", delivery_id: "delivery", output_kind: "assistant"});
  view.rerender(page());
  expect(screen.getByText("team.delivery_leased")).toBeTruthy();
  room.room!.deliveries = [{...delivery, state: "failed", failure_code: "lease_expired"}];
  view.rerender(page());
  expect(screen.getByText("team.delivery_expired")).toBeTruthy();
  room.room!.deliveries = [{...delivery, state: "failed", failure_code: "execution_failed"}];
  view.rerender(page());
  expect(screen.getByText("team.delivery_failed")).toBeTruthy();
  room.messages.push({...trigger, id: "final", author_type: "agent", author_agent_id: "agent", delivery_id: "delivery", output_kind: "final"});
  view.rerender(page());
  expect(screen.queryByText("team.delivery_failed")).toBeNull();
});
it("moves the local Thread action from its trigger to the Agent reply header", async () => {
  model.agents.mockResolvedValue([{agent_id: "agent", name: "Amy"}]);
  model.node.mockResolvedValue({state: "authorized", jobs: [{id: "job", agent_id: "agent", local_agent_id: "local-agent", source_room_id: "room", source_message_id: "trigger", delivery_id: "delivery", room_id: "local-room", conversation_id: "local-conversation", round_id: "round", state: "completed"}]});
  const trigger = {id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user" as const, author_user_id: "other", author_username: "Other", author_display_name: "Other", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "Request"}]}, created_at: "2026-09-09T01:00:00Z"};
  room.messages = [{...trigger, content: {version: 1, blocks: [{type: "markdown", text: "Request"}]}}, {...trigger, id: "reply", message_seq: 2, author_type: "agent", author_agent_id: "agent", delivery_id: "delivery", content: {version: 1, blocks: [{type: "markdown", text: "Result"}]}}];
  render(page());
  const thread = await screen.findByRole("button", {name: "room.thread_action_open"});
  expect(thread.closest(".nexus-chat-message-header")).toBeTruthy();
  expect(screen.getByText("Request").closest("li")?.querySelector('[data-room-agent-action="thread"]')).toBeNull();
  expect(screen.getAllByRole("button", {name: "room.thread_action_open"})).toHaveLength(1);
  const replyChip = screen.getByText("room.agent_handoff_reply");
  expect(replyChip.closest(".nexus-chat-message-header")).toBe(thread.closest(".nexus-chat-message-header"));
  expect(replyChip.closest("[data-reply-chip]")?.hasAttribute("data-handoff-reply")).toBe(false);
  expect(replyChip.closest("[data-reply-chip]")?.getAttribute("aria-label")).toContain("Request");
  expect(replyChip.closest("[data-reply-chip]")?.className).toContain("radius-control-sm");
  expect(replyChip.closest("[data-reply-chip]")?.className).not.toContain("rounded-full");
  expect(screen.queryByText("team.reply_to")).toBeNull();
});
it("shows the Agent execution identity before its first public reply", async () => {
  model.agents.mockResolvedValue([{agent_id: "agent", name: "Lucy", avatar: "/icon/agent/3.png"}]);
  model.node.mockResolvedValue({state: "authorized", jobs: [{id: "job", agent_id: "agent", local_agent_id: "local-agent", source_room_id: "room", source_message_id: "trigger", delivery_id: "delivery", room_id: "local-room", conversation_id: "local-conversation", round_id: "round", state: "running"}]});
  room.messages = [{id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user", author_user_id: "other", author_username: "Other", author_display_name: "Other", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "hey"}]}, created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  const thread = await screen.findByRole("button", {name: "room.thread_action_open"});
  expect(thread.closest(".nexus-chat-message-header")?.textContent).toContain("Lucy");
  expect(thread.closest("li")?.querySelector("img")?.getAttribute("src")).toBe("/icon/agent/3.png");
});
it("stops only the bound native execution and clears actions with its subscription", async () => {
  const binding = {agent_id: "agent", local_agent_id: "local-agent", room_id: "local-room", conversation_id: "local-conversation"};
  room.room!.members = [{room_id: "room", member_type: "agent", member_id: "agent", role: "member", state: "active", invited_by_user_id: "owner", joined_at: "", created_at: "", updated_at: ""}];
  model.prepare.mockResolvedValue([binding]);
  model.node.mockResolvedValue({jobs: [{id: "job", agent_id: "agent", local_agent_id: "local-agent", source_room_id: "room", source_message_id: "trigger", delivery_id: "delivery", room_id: "local-room", conversation_id: "local-conversation", round_id: "round", state: "running"}]});
  room.messages = [{id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user", author_user_id: "owner", author_username: "owner", author_display_name: "Owner", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "hey"}]}, created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  await waitFor(() => expect(model.observe).toHaveBeenCalled());
  const controls = {stop_generation: vi.fn(), send_permission_response: vi.fn().mockReturnValue(true), stopping_agent_round_ids: [],
    room_agent_execution_states: [{round_id: "round", agent_id: "local-agent", agent_round_id: "exact-execution", status: "streaming", phase: "active"}],
    pending_permissions: [{request_id: "permission", agent_id: "local-agent", round_id: "round", tool_name: "Bash", tool_input: {command: "pwd"}},
      {request_id: "foreign", agent_id: "other-agent", round_id: "round", tool_name: "Bash", tool_input: {command: "pwd"}}]};
  act(() => model.observe.mock.lastCall![0].onControls("local-conversation", controls));
  fireEvent.click(await screen.findByRole("button", {name: "room.agent_stop_action"}));
  expect(controls.stop_generation).toHaveBeenCalledExactlyOnceWith("exact-execution");
  expect(document.querySelector('[data-pending-interaction-request-id="permission"]')).toBeTruthy();
  expect(document.querySelector('[data-pending-interaction-request-id="foreign"]')).toBeNull();
  fireEvent.click(screen.getByRole("button", {name: "composer.permission_allow_once"}));
  expect(controls.send_permission_response).toHaveBeenCalledWith(expect.objectContaining({request_id: "permission", decision: "allow"}));
  act(() => model.observe.mock.lastCall![0].onControls("local-conversation", null));
  expect(screen.queryByRole("button", {name: "room.agent_stop_action"})).toBeNull();
  expect(document.querySelector('[data-pending-interaction-request-id="permission"]')).toBeNull();
});

it("does not send on composition confirmation or Shift+Enter, then sends exact text on Enter", async () => {
  render(page());
  const input = screen.getByPlaceholderText("team.message_placeholder");
  await userEvent.type(input, "中文草稿");
  fireEvent.keyDown(input, {key: "Enter", isComposing: true});
  fireEvent.keyDown(input, {key: "Enter", keyCode: 229});
  fireEvent.keyDown(input, {key: "Enter", shiftKey: true});
  expect(room.send).not.toHaveBeenCalled();
  await userEvent.keyboard("{Enter}");
  expect(room.send).toHaveBeenCalledExactlyOnceWith("中文草稿");
  expect((input as HTMLTextAreaElement).value).toBe("");
});

it("retains pasted online attachments on upload failure without sending text alone", async () => {
  render(page());
  const input = screen.getByPlaceholderText("team.message_placeholder");
  await userEvent.type(input, "draft");
  const file = new File(["image"], "image.png", {type: "image/png"});
  fireEvent.paste(input, {clipboardData: {items: [{kind: "file", type: "image/png", getAsFile: () => file}], files: [file], getData: () => ""}});
  await userEvent.keyboard("{Enter}");
  expect(await screen.findByText("composer.attachment_failed")).toBeTruthy();
  expect((input as HTMLTextAreaElement).value).toBe("draft");
  expect(room.send).not.toHaveBeenCalled();
  model.upload.mockResolvedValue({id:"file",name:"image.png",size:5,sha256:"a".repeat(64)});
  await userEvent.clear(input);
  await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(room.send).toHaveBeenCalledWith("", [], [{id:"file",name:"image.png",size:5,sha256:"a".repeat(64)}]));
});
it("retains the draft after a rejected send and refuses empty or busy submissions", async () => {
  room.send = vi.fn().mockResolvedValue(false);
  const view = render(page());
  const input = screen.getByPlaceholderText("team.message_placeholder");
  fireEvent.keyDown(input, {key: "Enter"});
  expect(room.send).not.toHaveBeenCalled();
  await userEvent.type(input, "Keep me");
  await userEvent.click(screen.getByRole("button", {name: "composer.send_message"}));
  expect((input as HTMLTextAreaElement).value).toBe("Keep me");
  room = {...room, isSending: true};
  view.rerender(page());
  fireEvent.keyDown(input, {key: "Enter"});
  expect(room.send).toHaveBeenCalledTimes(1);
});
it("announces loading and load failure without claiming an empty conversation", () => {
  room = {...room, room: null, isLoading: true};
  const view = render(page());
  expect(screen.getByRole("status").textContent).toBe("team.loading");
  room = {...room, isLoading: false, error: "load"};
  view.rerender(page());
  expect(screen.getByRole("alert").querySelector("[data-inline-notice-message]")?.textContent).toBe("team.error_load");
  expect(screen.queryByText("team.empty")).toBeNull();
});

it("keeps complete Unicode initials decorative while preserving author and Markdown content", () => {
  room.messages = [{id: "message", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "internal-user", author_username: "researcher",
    author_display_name: "👩‍🔬 Researcher", client_message_id: "client",
    content: {version: 1, blocks: [{type: "markdown", text: "**Research result**"}]},
    created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  expect(screen.getByText("👩‍🔬").closest('[aria-hidden="true"]')).toBeTruthy();
  expect(screen.getByText("👩‍🔬 Researcher")).toBeTruthy();
  expect(screen.getByText("Research result").tagName).toBe("STRONG");
  expect(screen.getByText("Research result").closest(".nexus-chat-user-content-shell")).toBeTruthy();
  expect(screen.getByRole("button", {name: "message.copy"})).toBeTruthy();
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
  const input = screen.getByPlaceholderText("team.message_placeholder");
  fireEvent.change(input, {target: {value: "hello"}});
  fireEvent.click(screen.getByRole("button", {name: "composer.send_message"}));
  await waitFor(() => expect(room.send).toHaveBeenCalledExactlyOnceWith("hello"));
});

it("sends selected active Agents as structured mention targets", async () => {
  model.agents.mockResolvedValue([]);
  const view = render(page());
  expect(screen.queryByRole("button", {name: "team.mention_agent"})).toBeNull();
  model.agents.mockResolvedValue([{agent_id: "agent-one", owner_user_id: "owner", name: "Amy", avatar: "/icon/agent/3.png"}]);
  room.room!.room.membership_version = 2;
  room.room!.members = [{
    room_id: "room", member_type: "agent", member_id: "agent-one", role: "member",
    state: "active", agent_owner_user_id: "owner", invited_by_user_id: "owner",
    joined_at: "2026-09-09T00:00:00Z", created_at: "2026-09-09T00:00:00Z",
    updated_at: "2026-09-09T00:00:00Z",
  }];
  view.rerender(page());
  await waitFor(() => expect(model.agents).toHaveBeenCalledTimes(2));
  await userEvent.type(screen.getByPlaceholderText("team.message_placeholder"), "@");
  expect((await screen.findByRole("option", {name: /Amy/})).querySelector("img")?.getAttribute("src")).toBe("/icon/agent/3.png");
  await userEvent.click(await screen.findByRole("option", {name: /Amy/}));
  await waitFor(() => expect((screen.getByPlaceholderText("team.message_placeholder") as HTMLTextAreaElement).selectionStart).toBe(5));
  await userEvent.type(screen.getByPlaceholderText("team.message_placeholder"), "分析任务");
  room.room!.room.membership_version = 3;
  room.room!.members = [];
  view.rerender(page());
  expect((screen.getByPlaceholderText("team.message_placeholder") as HTMLTextAreaElement).value).toContain("@Amy");
  await userEvent.click(screen.getByRole("button", {name: "composer.send_message"}));
  await waitFor(() => expect(room.send).toHaveBeenCalledExactlyOnceWith("@Amy 分析任务", ["agent-one"]));
});

it("uses the shared Slash picker and dispatches the command to an explicitly mentioned Agent", async () => {
  model.agents.mockResolvedValue([{agent_id:"agent-one",owner_user_id:"other",name:"Amy",avatar:"/icon/agent/3.png"}]);
  room.room!.members = [{room_id:"room",member_type:"agent",member_id:"agent-one",role:"member",state:"active",invited_by_user_id:"other",joined_at:null,created_at:"",updated_at:""}];
  render(page());
  await waitFor(() => expect(model.commands).toHaveBeenCalled());
  const input = screen.getByPlaceholderText("team.message_placeholder");
  await userEvent.type(input, "/");
  await userEvent.click(await screen.findByRole("option",{name:/browser/i}));
  await userEvent.type(input, "@");
  await userEvent.click(await screen.findByRole("option",{name:/Amy/}));
  await userEvent.type(input, "inspect");
  await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(room.send).toHaveBeenCalledWith("/browser @Amy inspect",["agent-one"]));
});

it("renders only explicit Relay mentions with Unicode offsets and whole-name boundaries", async () => {
  model.agents.mockResolvedValue([{agent_id: "amy", name: "Amy", avatar: "/icon/agent/3.png"}, {agent_id: "bob", name: "Bob"}]);
  room.messages = [{id: "mention", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "other", author_username: "Other", author_display_name: "Other",
    client_message_id: "mention", content: {version: 1, blocks: [{type: "markdown", text: "😀 @Amy hi @Amyson @Bob @Amy。"}]},
    mentions: [{member_type: "agent", member_id: "amy"}], created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  const chips = await screen.findAllByRole("button", {name: "room.agent_contact_open"});
  expect(chips).toHaveLength(2);
  for (const chip of chips) {
    expect(chip.textContent).toContain("@Amy");
    expect(chip.querySelector("img")?.getAttribute("src")).toBe("/icon/agent/3.png");
    expect((chip as HTMLButtonElement).disabled).toBe(true);
  }
});


it("preserves existing messages while a read refresh is pending", () => {
  room.messages = [{id: "existing", conversation_id: "conversation", message_seq: 1,
    author_type: "user", author_user_id: "user", author_username: "Name", author_display_name: "Name",
    client_message_id: "client", content: {version: 1, blocks: [{type: "markdown", text: "Keep visible"}]}, created_at: "2026-09-09T01:00:00Z"}];
  const view = render(page());
  expect(screen.getByText("Keep visible")).toBeTruthy();
  room = {...room, isLoading: true};
  view.rerender(page());
  expect(screen.getByText("Keep visible")).toBeTruthy();
  expect(screen.getByRole("list").getAttribute("aria-busy")).toBe("true");
  expect(screen.queryByText("team.loading")).toBeNull();
});


it("resets the draft on room navigation and ignores a previous room send completion", async () => {
  let resolve!: (value: boolean) => void;
  room.send = vi.fn(() => new Promise<boolean>(done => { resolve = done; }));
  render(page());
  const input = screen.getByPlaceholderText("team.message_placeholder");
  await userEvent.type(input, "Old room");
  await userEvent.click(screen.getByRole("button", {name: "composer.send_message"}));
  await userEvent.click(screen.getByRole("button", {name: "Switch room"}));
  const next = screen.getByPlaceholderText("team.message_placeholder");
  expect((next as HTMLTextAreaElement).value).toBe("");
  await userEvent.type(next, "New draft");
  resolve(true);
  await waitFor(() => expect((next as HTMLTextAreaElement).value).toBe("New draft"));
});

it("uses shared notices while keeping each recovery action scoped to its failed read", async () => {
  model.prepare.mockRejectedValueOnce(new Error("bindings unavailable"));
  model.node.mockRejectedValueOnce(new Error("jobs unavailable"));
  room = {...room, error: "sync"};
  render(page());
  await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(3));
  const binding = screen.getByText("team.binding_error").closest('[role="alert"]') as HTMLElement;
  const jobs = screen.getByText("team.node_jobs_error").closest('[role="alert"]') as HTMLElement;
  const sync = screen.getByText("team.error_sync").closest('[role="alert"]') as HTMLElement;
  expect(binding.dataset.inlineNoticeTone).toBe("danger");
  expect(jobs.dataset.inlineNoticeTone).toBe("danger");
  expect(sync.dataset.inlineNoticeTone).toBe("warning");
  expect(within(sync).queryByRole("button")).toBeNull();
  await userEvent.click(within(binding).getByRole("button", {name: "state.retry"}));
  await waitFor(() => expect(screen.queryByText("team.binding_error")).toBeNull());
  expect(screen.getByText("team.node_jobs_error")).toBeTruthy();
  await userEvent.click(within(jobs).getByRole("button", {name: "team.node_refresh"}));
  await waitFor(() => expect(screen.queryByText("team.node_jobs_error")).toBeNull());
  expect(room.send).not.toHaveBeenCalled();
  expect(room.retryLoad).not.toHaveBeenCalled();
});

it("discovers a local Thread from Room execution before an Agent reply and honors native cancellation", async () => {
  model.prepare.mockResolvedValue([{agent_id: "remote", local_agent_id: "local", room_id: "local-room", conversation_id: "local-conversation"}]);
  model.agents.mockResolvedValue([{agent_id: "remote", name: "Amy"}]);
  room.room!.members = [{room_id: "room", member_type: "agent", member_id: "remote", role: "member", state: "active", invited_by_user_id: "owner", joined_at: "", created_at: "", updated_at: ""}];
  room.messages = [{id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user", author_user_id: "owner", author_username: "Owner", author_display_name: "Owner", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "Request"}]}, created_at: "2026-09-09T01:00:00Z"}];
  render(page());
  await waitFor(() => expect(model.observe).toHaveBeenCalled());
  expect(screen.queryByRole("button", {name: "room.thread_action_open"})).toBeNull();
  model.node.mockResolvedValue({state: "authorized", jobs: [{id: "job", agent_id: "remote", local_agent_id: "local", source_room_id: "room", source_message_id: "trigger", delivery_id: "delivery", room_id: "local-room", conversation_id: "local-conversation", round_id: "round", state: "running"}]});
  const execution = {agent_id: "local", round_id: "round", agent_round_id: "execution", first_seen_at: 1, display_order: 1, phase: "active", status: "streaming"};
  act(() => model.observe.mock.lastCall![0].onChange("local-conversation", [execution]));
  expect(await screen.findByRole("button", {name: "room.thread_action_open"})).toBeTruthy();
  expect(room.messages).toHaveLength(1);
  act(() => model.observe.mock.lastCall![0].onChange("local-conversation", [{...execution, phase: "terminal", status: "cancelled"}]));
  expect(await screen.findByText("team.node_job_cancelled")).toBeTruthy();
});

it("uses my signed-in avatar for reply sources absent from the invitation directory", () => {
  const trigger: import("@/lib/api/conversation/team-api").TeamMessage = {id: "trigger", conversation_id: "conversation", message_seq: 1, author_type: "user" as const, author_user_id: "owner", author_username: "Admin", author_display_name: "Admin", client_message_id: "trigger", content: {version: 1, blocks: [{type: "markdown", text: "Request"}]}, created_at: "2026-09-09T01:00:00Z"};
  room.room!.deliveries = [{id: "delivery", message_id: "trigger", agent_id: "agent", state: "completed"}];
  room.messages = [trigger, {...trigger, id: "reply", message_seq: 2, author_type: "agent", author_agent_id: "agent", delivery_id: "delivery"}];
  const view = render(page("organization", "/icon/agent/3.png"));
  expect(view.container.querySelector('[data-reply-chip] img')?.getAttribute("src")).toBe("/icon/agent/3.png");
  room.messages = [{...trigger, author_type: "agent", author_agent_id: "other-agent"}, room.messages[1]];
  view.rerender(page("organization", "/icon/agent/3.png"));
  expect(view.container.querySelector('[data-reply-chip] img')?.getAttribute("src")).not.toBe("/icon/agent/3.png");
});
