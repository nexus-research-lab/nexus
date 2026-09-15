// INPUT: Relay Room 成员快照和邀请命令。
// OUTPUT: mutation 使用当前 membership_version，并在成功后读取新快照。
// POS: Team 成员治理资源的最小并发合同。
import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { useTeamRoomMembers } from "./use-team-room-members";
import { advanceAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";

const api = vi.hoisted(() => ({ add: vi.fn(), publish: vi.fn(), coordinator: vi.fn(), get: vi.fn(), invite: vi.fn(), pause: vi.fn() }));
vi.mock("@/lib/api/account/control-api", () => ({ publishControlAgentApi: api.publish }));
vi.mock("@/lib/api/conversation/team-api", () => ({
	addTeamRoomAgent: api.add,
  getTeamRoom: api.get,
  inviteTeamRoomMember: api.invite,
	removeTeamRoomAgent: vi.fn(),
  revokeTeamRoomInvitation: vi.fn(),
  transferTeamRoomOwnership: vi.fn(),
	updateTeamRoomCoordinator: api.coordinator,
	updateTeamRoomAgent: api.pause,
  updateTeamRoomMember: vi.fn(),
}));

it("uses independent member and configuration versions", async () => {
  const details = (membershipVersion: number, configurationVersion: number) => ({ room: { configuration_version: configurationVersion, id: "room-1", membership_version: membershipVersion }, members: [] });
  api.get.mockResolvedValueOnce(details(3, 7)).mockResolvedValueOnce(details(4, 7)).mockResolvedValueOnce(details(4, 8)).mockResolvedValueOnce(details(5, 9));
  api.invite.mockResolvedValue({ room_id: "room-1", membership_version: 4, replayed: false });
	api.coordinator.mockResolvedValue({ room_id: "room-1", configuration_version: 8, replayed: false });
	api.pause.mockResolvedValue({ room_id: "room-1", membership_version: 5, replayed: false });
  const onChanged = vi.fn();
  const { result } = renderHook(() => useTeamRoomMembers("room-1", true, onChanged));
  await waitFor(() => expect(result.current.details?.room.membership_version).toBe(3));

  await act(async () => { expect(await result.current.invite("user-2")).toBe(true); });

  expect(api.invite).toHaveBeenCalledWith("room-1", "user-2", 3, expect.any(String));
  expect(result.current.details?.room.membership_version).toBe(4);

	await act(async () => { expect(await result.current.setCoordinator("agent-2")).toBe(true); });

	expect(api.coordinator).toHaveBeenCalledWith("room-1", "agent-2", 7, expect.any(String));
	expect(result.current.details?.room.configuration_version).toBe(8);

	await act(async () => { expect(await result.current.setAgentPaused("agent-2", true)).toBe(true); });

	expect(api.pause).toHaveBeenCalledWith("room-1", "agent-2", true, 4, expect.any(String));
	expect(result.current.details?.room.membership_version).toBe(5);
  expect(onChanged).toHaveBeenLastCalledWith(details(5, 9));
});

it("guards publication and addition together, surfaces failure and retries without double publication", async () => {
  api.get.mockReset().mockResolvedValue({room: {id: "room-1", membership_version: 3}, members: []});
  const agent = {id: "local-agent", name: "Amy"} as Parameters<ReturnType<typeof useTeamRoomMembers>["addAgent"]>[0];
  let reject!: (error: Error) => void;
  api.publish.mockReset().mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
  api.add.mockReset().mockResolvedValue({});
  const {result} = renderHook(() => useTeamRoomMembers("room-1", true));
  await waitFor(() => expect(result.current.details).not.toBeNull());
  let request!: Promise<boolean>;
  act(() => { request = result.current.addAgent(agent); void result.current.addAgent(agent); });
  expect(result.current.busy).toBe(true);
  expect(api.publish).toHaveBeenCalledOnce();
  expect(api.add).not.toHaveBeenCalled();
  await act(async () => { reject(new Error("publish unavailable")); expect(await request).toBe(false); });
  expect(result.current.failed).toBe(true);
  expect(result.current.busy).toBe(false);
  api.publish.mockResolvedValueOnce({agent_id: "published-agent"});
  await act(async () => { expect(await result.current.addAgent(agent)).toBe(true); });
  expect(api.add).toHaveBeenCalledExactlyOnceWith("room-1", "published-agent", 3, expect.any(String));
  expect(result.current.failed).toBe(false);
});

it("does not add an Agent under a different account after publication completes late", async () => {
  api.get.mockReset().mockResolvedValue({room: {id: "room-1", membership_version: 3}, members: []});
  let finish!: (agent: {agent_id: string}) => void;
  api.publish.mockReset().mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  api.add.mockReset();
  const {result} = renderHook(() => useTeamRoomMembers("room-1", true));
  await waitFor(() => expect(result.current.details).not.toBeNull());
  let request!: Promise<boolean>;
  act(() => { request = result.current.addAgent({id: "local", name: "Amy"} as Parameters<typeof result.current.addAgent>[0]); });
  advanceAuthOwnerScopeGeneration();
  await act(async () => { finish({agent_id: "remote"}); expect(await request).toBe(false); });
  expect(api.add).not.toHaveBeenCalled();
});
