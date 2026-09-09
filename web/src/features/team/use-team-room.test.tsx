// INPUT: Failed Team directory reads and explicit retry/unmount commands.
// OUTPUT: Retry deduplication, cancellation and no message replay evidence.
// POS: Team resource hook tests with isolated transport boundaries.
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useTeamRoom } from "./use-team-room";
const api = vi.hoisted(() => ({bootstrap: vi.fn(), snapshot: vi.fn(), post: vi.fn()}));
vi.mock("@/lib/api/conversation/team-api", () => ({
  listTeamRooms: api.bootstrap, getTeamSnapshot: api.snapshot, postTeamMessage: api.post,
  buildTeamStreamUrl: () => "", getTeamDifference: vi.fn(),
}));
vi.mock("@/lib/websocket/use-socket", () => ({useWebSocket: vi.fn()}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/shared/auth/auth-context")>(),
  useAuth: () => ({status: {authenticated: true, auth_method: "password"}}),
}));
const bootstrap = {
  team: {id: "team", deployment_id: "deployment", name: "Team"},
  room: {id: "room", team_id: "team", name: "General"},
  conversation: {id: "conversation", room_id: "room", type: "team", high_water_message_seq: 0,
    sync_stream_id: "stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
};
beforeEach(() => {
  api.bootstrap.mockReset().mockRejectedValueOnce(new Error("unavailable"));
  api.snapshot.mockReset().mockResolvedValue({messages: [], snapshot_seq: 0, has_more: false});
  api.post.mockReset();
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
