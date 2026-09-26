// INPUT: Failed Team directory reads and explicit retry/unmount commands.
// OUTPUT: Retry deduplication, cancellation and no message replay evidence.
// POS: Team resource hook tests with isolated transport boundaries.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useTeamRoom } from "./use-team-room";
import { ApiRequestError } from "@/lib/api/core/http-error";
const api = vi.hoisted(() => ({bootstrap: vi.fn(), get: vi.fn(), snapshot: vi.fn(), post: vi.fn(), difference: vi.fn(), socket: vi.fn(), markRead: vi.fn(), deliveries: vi.fn()}));
vi.mock("@/lib/api/conversation/team-api", () => ({
  getTeamDeliveryStatuses: api.deliveries, markTeamRoomRead: api.markRead, listTeamRooms: api.bootstrap, getTeamRoom: api.get, getTeamSnapshot: api.snapshot, postTeamMessage: api.post,
  buildTeamStreamUrl: () => "", getTeamDifference: api.difference,
}));
vi.mock("@/lib/websocket/use-socket", () => ({useWebSocket: api.socket}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/shared/auth/auth-context")>(),
  useAuth: () => ({status: {authenticated: true, auth_method: "password", control_user_id: "user", organization_id: "org"}}),
}));
const bootstrap = {
  last_read_message_seq: 0,
  team: {id: "team", deployment_id: "deployment", name: "Team"},
  room: {id: "room", team_id: "team", name: "General", membership_version: 7, configuration_version: 1},
  conversation: {id: "conversation", room_id: "room", type: "team", high_water_message_seq: 0,
    sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
  members: [],
};
it("refreshes shared delivery state even when the stream message cursor does not advance", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const { result } = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(result.current.room?.room.id).toBe("room"));
  const deliveries = [{id: "delivery", message_id: "message", agent_id: "remote", state: "leased"}];
  api.get.mockResolvedValue({...bootstrap, deliveries});
  await act(async () => { api.socket.mock.calls.at(-1)![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 0}); });
  await waitFor(() => expect(result.current.room?.deliveries).toEqual(deliveries));
});
beforeEach(() => {
  localStorage.clear();
  api.bootstrap.mockReset().mockRejectedValueOnce(new Error("unavailable"));
  api.snapshot.mockReset().mockResolvedValue({messages: [], snapshot_seq: 0, has_more: false});
	api.get.mockReset().mockResolvedValue(bootstrap);
  api.post.mockReset();
  api.deliveries.mockReset().mockResolvedValue([]);
  api.markRead.mockReset().mockResolvedValue({last_read_message_seq: 0});
  api.difference.mockReset();
  api.socket.mockClear();
});

it("重连初始水位按旧游标分页补齐，重复提示不重发消息", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  const first = {id: "offline-1", message_seq: 1, author_type: "user", author_user_id: "other"};
  const second = {...first, id: "offline-2", message_seq: 2};
  api.difference.mockResolvedValueOnce({events: [{message: first}], next_seq: 1, high_water_seq: 2});
  api.difference.mockResolvedValueOnce({events: [{message: second}], next_seq: 2, high_water_seq: 2});
  const hint = {type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 2};
  await act(async () => { api.socket.mock.lastCall![0].onMessage(hint); });
  await waitFor(() => expect(result.current.messages).toEqual([first, second]));
  expect(api.difference.mock.calls).toEqual([["stream", 0, "epoch"], ["stream", 1, "epoch"]]);
  await act(async () => { api.socket.mock.lastCall![0].onMessage(hint); });
  expect(api.difference).toHaveBeenCalledTimes(2);
  expect(api.snapshot).toHaveBeenCalledTimes(1);
  expect(api.post).not.toHaveBeenCalled();
  // 不覆盖共享传输的心跳配置，在线 Room 与本地 Room 使用同一策略。
  expect(api.socket.mock.lastCall![0].heartbeatInterval).toBeUndefined();
});

it("服务端换代后重建快照，不携带旧世代游标或重放写入", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  const next = {...bootstrap, conversation: {...bootstrap.conversation, stream_epoch: "new-epoch"}};
  const message = {id: "restored", message_seq: 1};
  api.bootstrap.mockResolvedValue({rooms: [next]});
  api.get.mockResolvedValue(next);
  api.snapshot.mockResolvedValue({messages: [message], snapshot_seq: 1, has_more: false});
  await act(async () => { api.socket.mock.lastCall![0].onMessage({type: "stream.reset_required", stream_id: "stream", reason: "full_snapshot_required"}); });
  await waitFor(() => expect(result.current.messages).toEqual([message]));
  expect(result.current.room?.conversation.stream_epoch).toBe("new-epoch");
  expect(api.snapshot).toHaveBeenCalledTimes(2);
  expect(api.post).not.toHaveBeenCalled();
});

it("coalesces delivery hints while a room detail read is pending", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  let finish!: (value: typeof bootstrap) => void;
  api.get.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const hint = {type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 0};
  act(() => {
    for (let i = 0; i < 8; i++) api.socket.mock.lastCall![0].onMessage(hint);
  });
  expect(api.get).toHaveBeenCalledTimes(3);
  await act(async () => { finish(bootstrap); });
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(4));
  expect(result.current.error).toBeNull();
  expect(api.snapshot).toHaveBeenCalledTimes(1);
});

it("does not revoke a new room when the previous room difference fails late", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result, rerender} = renderHook(({id}) => useTeamRoom(id), {initialProps: {id: "room"}});
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  let reject!: (cause: Error) => void;
  api.difference.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
  act(() => api.socket.mock.lastCall![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 1}));
  const next = {...bootstrap, room: {...bootstrap.room, id: "other"}, conversation: {...bootstrap.conversation, id: "other-conversation", room_id: "other"}};
  api.bootstrap.mockResolvedValue({rooms: [next]});
  api.get.mockResolvedValue(next);
  rerender({id: "other"});
  await waitFor(() => expect(result.current.room?.room.id).toBe("other"));
  const message = {id: "new-room-message", message_seq: 1, author_type: "user", author_user_id: "other"};
  api.difference.mockResolvedValue({events: [{message}], next_seq: 1, high_water_seq: 1});
  act(() => api.socket.mock.lastCall![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 1}));
  await waitFor(() => expect(result.current.messages).toEqual([message]));
  await act(async () => { reject(new ApiRequestError("previous room revoked", 403)); });
  expect(result.current.room?.room.id).toBe("other");
  expect(result.current.error).toBeNull();
});

it("ignores an obsolete detail failure after a newer send recovery snapshot", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  let reject!: (cause: Error) => void;
  api.get.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
  act(() => window.dispatchEvent(new Event("focus")));
  api.post.mockRejectedValueOnce(new Error("lost receipt"));
  await act(async () => { await result.current.send("hello"); });
  await act(async () => { reject(new ApiRequestError("stale denied", 403)); });
  expect(result.current.room?.room.id).toBe("room");
  expect(result.current.error).toBe("send");
  expect(result.current.hasUnconfirmedSend).toBe(true);
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
    undefined,
  );
});

it("replays the frozen intent after an unknown result and accepts refreshed membership only for a new command", async () => {
  const files = [{id:"file-one",name:"report.txt",size:3,sha256:"a".repeat(64)}];
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result} = renderHook(() => useTeamRoom(null));
  await waitFor(() => expect(result.current.room).toEqual(bootstrap));
  api.get.mockResolvedValue({...bootstrap, room: {...bootstrap.room, membership_version: 8}});
  api.post.mockRejectedValueOnce(new Error("response lost"));
  await act(async () => { expect(await result.current.send("@Amy 原始任务", ["agent-one"], files)).toBe(false); });
  expect(api.post.mock.calls[0][4]).toEqual(files);
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

it("只读最近一页，历史前插保留并发新消息和实时游标", async () => {
  const value = {...bootstrap, conversation: {...bootstrap.conversation, high_water_message_seq: 250, high_water_sync_event_seq: 250}};
  api.bootstrap.mockReset().mockResolvedValue({rooms: [value]});
  api.get.mockResolvedValue(value);
  const message = (seq: number) => ({id: `m${seq}`, message_seq: seq, author_type: "user", author_user_id: "other"});
  api.snapshot.mockResolvedValueOnce({messages: Array.from({length: 100}, (_, i) => message(151 + i)), snapshot_seq: 250, through_message_seq: 250, has_more: false});
  const {result} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(result.current.messages).toHaveLength(100));
  expect(Object.fromEntries(api.snapshot.mock.calls[0][1])).toEqual({after_message_seq: "150", through_message_seq: "250", snapshot_seq: "250", stream_epoch: "epoch", limit: "100"});
  let finish!: (value: unknown) => void;
  api.snapshot.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const preparePrepend = vi.fn();
  const historyStatus = {id: "old-delivery", message_id: "m51", agent_id: "remote", state: "completed"};
  api.deliveries.mockImplementation(async (_room, ids) => ids.includes("m51") ? [historyStatus] : []);
  let loading!: Promise<boolean>;
  act(() => { loading = result.current.loadEarlier(preparePrepend); });
  await act(async () => { expect(await result.current.loadEarlier()).toBe(false); });
  expect(preparePrepend).not.toHaveBeenCalled();
  api.difference.mockResolvedValueOnce({events: [{message: message(251)}], next_seq: 251, high_water_seq: 251});
  await act(async () => { api.socket.mock.lastCall![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 251}); });
  await act(async () => { finish({messages: Array.from({length: 100}, (_, i) => message(51 + i)), snapshot_seq: 250}); await loading; });
  expect(preparePrepend).toHaveBeenCalledOnce();
  expect(result.current.messages).toHaveLength(201);
  expect(result.current.messages.at(-1)?.message_seq).toBe(251);
  expect(result.current.historyPrependToken).toBe(1);
  expect(result.current.room?.deliveries).toContainEqual(historyStatus);
  expect(Object.fromEntries(api.snapshot.mock.calls[1][1])).toEqual({after_message_seq: "50", through_message_seq: "250", snapshot_seq: "250", stream_epoch: "epoch", limit: "100"});
  api.snapshot.mockResolvedValueOnce({messages: Array.from({length: 50}, (_, i) => message(1 + i)), snapshot_seq: 250});
  await act(async () => { await result.current.loadEarlier(); });
  expect(result.current.hasEarlier).toBe(false);
  expect(result.current.messages).toHaveLength(251);
  expect(api.snapshot.mock.calls[2][1].get("limit")).toBe("50");
  api.difference.mockResolvedValueOnce({events: [], next_seq: 252, high_water_seq: 252});
  await act(async () => { api.socket.mock.lastCall![0].onMessage({type: "stream.updated", stream_id: "stream", stream_epoch: "epoch", high_water_seq: 252}); });
  expect(api.difference.mock.lastCall?.[1]).toBe(251);
});

it("历史读取失败可以重试，卸载取消读取且迟到结果不生效", async () => {
  const value = {...bootstrap, conversation: {...bootstrap.conversation, high_water_message_seq: 101, high_water_sync_event_seq: 101}};
  api.bootstrap.mockReset().mockResolvedValue({rooms: [value]});
  api.get.mockResolvedValue(value);
  api.snapshot.mockResolvedValueOnce({messages: [{id: "m101", message_seq: 101}], snapshot_seq: 101});
  const {result, unmount} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(result.current.hasEarlier).toBe(true));
  api.snapshot.mockRejectedValueOnce(new Error("offline"));
  await act(async () => { expect(await result.current.loadEarlier()).toBe(false); });
  expect(result.current.historyError).toBe(true);
  expect(result.current.messages).toHaveLength(1);
  let finish!: (value: unknown) => void;
  api.snapshot.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  let loading!: Promise<boolean>;
  act(() => { loading = result.current.loadEarlier(); });
  const signal = api.snapshot.mock.lastCall![2] as AbortSignal;
  unmount();
  expect(signal.aborted).toBe(true);
  await act(async () => { finish({messages: []}); expect(await loading).toBe(false); });
});

it("阅读确认使用当前会话世代、单飞且不回退水位", async () => {
  api.bootstrap.mockReset().mockResolvedValue({rooms: [bootstrap]});
  const {result, unmount} = renderHook(() => useTeamRoom("room"));
  await waitFor(() => expect(result.current.room?.room.id).toBe("room"));
  let finish!: (value: unknown) => void;
  api.markRead.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  let reading!: Promise<void>;
  act(() => { reading = result.current.markRead(10); });
  await act(async () => { await result.current.markRead(12); });
  expect(api.markRead).toHaveBeenCalledOnce();
  expect(api.markRead.mock.calls[0].slice(0, 3)).toEqual(["room", 10, "epoch"]);
  api.markRead.mockResolvedValueOnce({last_read_message_seq: 12});
  await act(async () => { finish({last_read_message_seq: 10}); await reading; });
  await act(async () => { await result.current.markRead(9); });
  expect(api.markRead).toHaveBeenCalledTimes(2);
  expect(api.markRead.mock.lastCall?.[1]).toBe(12);
  expect(result.current.room?.last_read_message_seq).toBe(12);
  api.markRead.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  act(() => { reading = result.current.markRead(13); });
  const signal = api.markRead.mock.lastCall![3] as AbortSignal;
  unmount();
  expect(signal.aborted).toBe(true);
  await act(async () => { finish({last_read_message_seq: 13}); await reading; });
});
