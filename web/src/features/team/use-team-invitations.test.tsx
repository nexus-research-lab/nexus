// INPUT: 远程账号的 pending Room 邀请与接受结果。
// OUTPUT: 接受命令携带当前成员版本，成功后刷新在线 Room 目录。
// POS: Team 邀请资源的最小交互合同。
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import type { TeamRoomInvitation } from "@/lib/api/conversation/team-api";

import { useTeamInvitations } from "./use-team-invitations";
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

const api = vi.hoisted(() => ({ list: vi.fn(), resolve: vi.fn(), recover: vi.fn() }));
vi.mock("@/lib/api/conversation/team-api", () => ({
  listTeamInvitations: api.list,
  resolveTeamRoomInvitation: api.resolve,
  transferTeamRoomOwnership: api.recover,
}));
vi.mock("@/shared/auth/auth-context", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/shared/auth/auth-context")>(),
  useAuth: () => ({ status: { authenticated: true, auth_method: "password", control_user_id: "owner" } }),
}));

it("accepts the current invitation version and refreshes the Room directory", async () => {
  const invitation = { room: { id: "room-1", membership_version: 3 }, invited_by_user_id: "owner", created_at: "2026-09-14T00:00:00Z" } as TeamRoomInvitation;
  api.list.mockResolvedValue({ invitations: [invitation] });
  api.resolve.mockResolvedValue({ room_id: "room-1", membership_version: 4, replayed: false });
  const accepted = vi.fn();
  const { result } = renderHook(() => useTeamInvitations(accepted));
  await waitFor(() => expect(result.current.invitations).toHaveLength(1));

  await act(async () => { await result.current.resolve(invitation, "accept"); });

  expect(api.resolve).toHaveBeenCalledWith("room-1", 3, "accept", expect.any(String));
  expect(result.current.invitations).toEqual([]);
  expect(accepted).toHaveBeenCalledOnce();
});

it("discovers late invitations on visible refresh, retains data on failure and stops polling when hidden", async () => {
  api.list.mockReset().mockResolvedValue({invitations: []});
  const {result, unmount} = renderHook(() => useTeamInvitations(vi.fn()));
  await waitFor(() => expect(result.current.loading).toBe(false));
  vi.useFakeTimers();
  const invitation = {room: {id: "late-room", membership_version: 2}} as TeamRoomInvitation;
  api.list.mockResolvedValue({invitations: [invitation]});
  await act(async () => { window.dispatchEvent(new Event("focus")); });
  expect(result.current.invitations).toEqual([invitation]);
  api.list.mockRejectedValueOnce(new Error("offline"));
  await act(async () => { result.current.refresh(); result.current.refresh(); });
  expect(api.list).toHaveBeenCalledTimes(3);
  expect(result.current.failed).toBe(true);
  expect(result.current.invitations).toEqual([invitation]);
  const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(api.list).toHaveBeenCalledTimes(3);
  visibility.mockReturnValue("visible");
  await act(async () => { document.dispatchEvent(new Event("visibilitychange")); });
  expect(api.list).toHaveBeenCalledTimes(4);
  expect(result.current.failed).toBe(false);
  unmount();
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); window.dispatchEvent(new Event("online")); });
  expect(api.list).toHaveBeenCalledTimes(4);
});

it("refreshes joined rooms after an accepted invitation loses its response and retries takeover exactly", async () => {
  const invitation = {room: {id: "room", membership_version: 1}} as TeamRoomInvitation;
  const recovery = {id: "orphan", name: "Orphan", membership_version: 5};
  api.list.mockReset().mockResolvedValue({invitations: [invitation], recovery_rooms: [recovery]});
  api.resolve.mockReset().mockRejectedValueOnce(new Error("lost"));
  api.recover.mockReset().mockRejectedValueOnce(new Error("lost")).mockResolvedValueOnce({});
  const joined = vi.fn();
  const {result} = renderHook(() => useTeamInvitations(joined));
  await waitFor(() => expect(result.current.recoveryRooms).toEqual([recovery]));
  await act(async () => { await result.current.resolve(invitation, "accept"); });
  expect(joined).toHaveBeenCalledOnce();
  await act(async () => { expect(await result.current.recover(recovery)).toBe(false); });
  await act(async () => { expect(await result.current.recover({...recovery, membership_version: 6})).toBe(true); });
  expect(api.recover.mock.calls[1]).toEqual(api.recover.mock.calls[0]);
  expect(api.recover).toHaveBeenCalledWith("orphan", "owner", 5, expect.any(String), true);
  expect(result.current.recoveryRooms).toEqual([]);
});
