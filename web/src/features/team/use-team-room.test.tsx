// INPUT: Failed Team directory reads and explicit retry/unmount commands.
// OUTPUT: Retry deduplication, cancellation and no message replay evidence.
// POS: Team resource hook tests with isolated transport boundaries.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useTeamRoom } from "./use-team-room";
import { ApiRequestError } from "@/lib/api/core/http-error";
const api = vi.hoisted(() => ({bootstrap: vi.fn(), get: vi.fn(), snapshot: vi.fn(), post: vi.fn(), difference: vi.fn(), socket: vi.fn()}));
vi.mock("@/lib/api/conversation/team-api", () => ({
  listTeamRooms: api.bootstrap, getTeamRoom: api.get, getTeamSnapshot: api.snapshot, postTeamMessage: api.post,
  buildTeamStreamUrl: () => "", getTeamDifference: api.difference,
}));
vi.mock("@/lib/websocket/use-socket", () => ({useWebSocket: api.socket}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/shared/auth/auth-context")>(),
  useAuth: () => ({status: {authenticated: true, auth_method: "password", control_user_id: "user", organization_id: "org"}}),
}));
const bootstrap = {
  team: {id: "team", deployment_id: "deployment", name: "Team"},
  room: {id: "room", team_id: "team", name: "General", membership_version: 7, configuration_version: 1},
  conversation: {id: "conversation", room_id: "room", type: "team", high_water_message_seq: 0,
    sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
  members: [],
};
beforeEach(() => {
  localStorage.clear();
  api.bootstrap.mockReset().mockRejectedValueOnce(new Error("unavailable"));
  api.snapshot.mockReset().mockResolvedValue({messages: [], snapshot_seq: 0, has_more: false});
	api.get.mockReset().mockResolvedValue(bootstrap);
  api.post.mockReset();
  api.difference.mockReset();
  api.socket.mockClear();
});

it("在推送补拉失败后通过焦点刷新水位补齐消息，不重载历史或重发消息", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  const message = {id: "remote-message", message_seq: 1, author_type: "user", author_user_id: "other"};
  api.difference.mockRejectedValueOnce(new Error("temporary failure"));
  act(() => api.socket.mock.lastCall![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 1}));
  await waitFor(() => expect(result.current.error).toBe("sync"));
  api.get.mockResolvedValue({...bootstrap, conversation: {...bootstrap.conversation, high_water_sync_event_seq: 1}});
  api.difference.mockResolvedValue({events: [{message}], next_seq: 1, high_water_seq: 1});
  act(() => window.dispatchEvent(new Event("focus")));
  await waitFor(() => expect(result.current.messages).toEqual([message]));
  expect(result.current.error).toBeNull();
  expect(api.difference).toHaveBeenLastCalledWith("stream", 0, "epoch");
  expect(api.snapshot).toHaveBeenCalledTimes(1);
  expect(api.post).not.toHaveBeenCalled();
});
it("deduplicates explicit retries and clears the load error only after a successful read", async () => {
  const {result} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.error).toBe("load"));
  let finish!: (value: {rooms: typeof bootstrap[]}) => void;
  api.bootstrap.mockImplementationOnce(() => new Promise((resolve) => {finish = resolve;}));
  let request!: Promise<void>;
  act(() => { request = result.current.retryLoad(); void result.current.retryLoad(); });
  expect(api.bootstrap).toHaveBeenCalledTimes(2);
  expect(result.current.isLoading).toBe(true);
  await act(async () => {finish({rooms: [bootstrap]}); await request;});
  expect(result.current.room).toEqual(bootstrap);
  expect(result.current.error).toBeNull();
  expect(result.current.isLoading).toBe(false);
  expect(api.post).not.toHaveBeenCalled();
});
it("aborts an outstanding retry when the hook unmounts", async () => {
  const {result, unmount} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.error).toBe("load"));
  let signal!: AbortSignal;
  api.bootstrap.mockImplementationOnce((nextSignal: AbortSignal) => {
    signal = nextSignal;
    return new Promise((_, reject) => nextSignal.addEventListener("abort", () => reject(new Error("aborted"))));
  });
  let request!: Promise<void>;
  act(() => {request = result.current.retryLoad();});
  unmount();
  await act(async () => {await request;});
  expect(signal.aborted).toBe(true);
  expect(api.post).not.toHaveBeenCalled();
});

it("posts exact Agent targets with the loaded membership fence", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  api.post.mockResolvedValue({message: {id: "message", message_seq: 1}});
  const {result} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.room).toEqual(bootstrap));
  await act(async () => { await result.current.send(" @Amy 分析 ", ["agent-one", "agent-one"]); });
  expect(api.post).toHaveBeenCalledExactlyOnceWith(
    "conversation", "@Amy 分析", expect.any(String),
    {agentIds: ["agent-one"], expectedMembershipVersion: 7},
  );
});

it("replays the frozen intent after an unknown result and accepts refreshed membership only for a new command", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.room).toEqual(bootstrap));
  api.get.mockResolvedValue({...bootstrap, room: {...bootstrap.room, membership_version: 8}});
  api.post.mockRejectedValueOnce(new Error("response lost"));
  await act(async () => { expect(await result.current.send("@Amy 原始任务", ["agent-one"])).toBe(false); });
  expect(result.current.room?.room.membership_version).toBe(8);
  expect(result.current.hasUnconfirmedSend).toBe(true);
  api.post.mockResolvedValueOnce({message: {id: "message", message_seq: 1}});
  await act(async () => { expect(await result.current.send("@Amy 原始任务", ["agent-one"])).toBe(true); });
  expect(api.post.mock.calls[1]).toEqual(api.post.mock.calls[0]);
  expect(result.current.hasUnconfirmedSend).toBe(false);
  api.post.mockResolvedValueOnce({message: {id: "message2", message_seq: 2}});
  await act(async () => { await result.current.send("新任务", ["agent-two"]); });
  expect(api.post.mock.calls[2][2]).not.toBe(api.post.mock.calls[0][2]);
  expect(api.post.mock.calls[2][3]).toEqual({agentIds: ["agent-two"], expectedMembershipVersion: 8});
});

it("refreshes a rejected membership fence and rotates only the definitely unapplied command", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.room).toEqual(bootstrap));
  api.get.mockResolvedValue({...bootstrap, room: {...bootstrap.room, membership_version: 9}});
  api.post.mockRejectedValueOnce(new ApiRequestError("membership changed", 409, {
    version: 1, code: "team.membership_version_conflict", category: "conflict", effect: "not_applied",
  }));
  await act(async () => { await result.current.send("任务", ["agent-one"]); });
  expect(result.current.hasUnconfirmedSend).toBe(false);
  api.post.mockResolvedValueOnce({message: {id: "message", message_seq: 1}});
  await act(async () => { await result.current.send("任务", ["agent-one"]); });
  expect(api.post.mock.calls[1][2]).not.toBe(api.post.mock.calls[0][2]);
  expect(api.post.mock.calls[1][3]).toEqual({agentIds: ["agent-one"], expectedMembershipVersion: 9});
});

it("recovers an exact persisted send across remount and reconciles a lost receipt from snapshot", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  api.post.mockRejectedValueOnce(new Error("response lost"));
  const first = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(first.result.current.room).not.toBeNull());
  await act(async () => { await first.result.current.send("@Amy original", ["agent-one"]); });
  const original = api.post.mock.calls[0];
  first.unmount();
  const second = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(second.result.current.pendingText).toBe("@Amy original"));
  expect(api.post).toHaveBeenCalledTimes(1);
  api.post.mockRejectedValueOnce(new Error("still unknown"));
  await act(async () => { await second.result.current.send("", []); });
  expect(api.post.mock.calls[1]).toEqual(original);
  api.post.mockRejectedValueOnce(new ApiRequestError("identity rejected", 502, {
    version: 1, code: "team.identity_rejected", category: "unavailable", effect: "not_applied",
  }));
  await act(async () => { await second.result.current.send(""); });
  expect(second.result.current.hasUnconfirmedSend).toBe(true);
  expect(api.post.mock.calls[2]).toEqual(original);
  second.unmount();
  api.snapshot.mockResolvedValue({messages: [{id: "agent-output", message_seq: 1, author_type: "agent", author_user_id: "user", client_message_id: original[2]}], snapshot_seq: 1, has_more: false});
  const agentSnapshot = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(agentSnapshot.result.current.messages).toHaveLength(1));
  expect(agentSnapshot.result.current.hasUnconfirmedSend).toBe(true);
  agentSnapshot.unmount();
  api.snapshot.mockResolvedValue({messages: [{id: "saved", message_seq: 2, author_type: "user", author_user_id: "user", client_message_id: original[2]}], snapshot_seq: 2, has_more: false});
  const third = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(third.result.current.messages).toHaveLength(1));
  expect(third.result.current.hasUnconfirmedSend).toBe(false);
  expect(localStorage.length).toBe(0);
  expect(api.post).toHaveBeenCalledTimes(3);
});

it("fails closed before sending if persistence is unavailable and drops a revoked room on refresh", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(result.current.room).not.toBeNull());
  const storage = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("quota"); });
  await act(async () => { expect(await result.current.send("unsafe")).toBe(false); });
  expect(api.post).not.toHaveBeenCalled();
  storage.mockRestore();
  api.bootstrap.mockResolvedValue({rooms: []});
  await act(async () => { await result.current.retryLoad(); });
  expect(result.current.room).toBeNull();
  await act(async () => { expect(await result.current.send("revoked")).toBe(false); });
  expect(api.post).not.toHaveBeenCalled();
});
