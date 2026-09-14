// INPUT: Persisted room membership plus a refreshed Agent directory.
// OUTPUT: Current avatar/profile projection without reordering or adding room members.
// POS: Room member read-model regression; no API or membership mutations.
import { expect, it } from "vitest";
import type { Agent } from "@/types/agent/agent";
import type { RoomContextAggregate } from "@/types/conversation/room";
import { resolveCurrentRoomMemberAgents } from "./room-member-model";

it("merges refreshed agent pictures by identity without changing room membership order", () => {
  const agent = (id: string, avatar: string): Agent => ({
    agent_id: id, name: id, avatar, created_at: 1, options: {}, status: "idle", workspace_path: "/workspace",
  });
  const contexts = [{
    member_agents: [agent("b", "/old-b.png"), agent("a", "/a.png")],
    members: [
      { member_type: "agent", member_agent_id: "b", participation_paused: true },
      { member_type: "agent", member_agent_id: "a", participation_paused: false },
    ],
  }] as unknown as RoomContextAggregate[];
  const projected = resolveCurrentRoomMemberAgents(contexts, [agent("a", "/new-a.png"), agent("other", "/other.png"), agent("b", "/new-b.png")]);
  expect(projected.map(({ agent_id, avatar, room_participation_paused }) => ({ agent_id, avatar, room_participation_paused }))).toEqual([
    { agent_id: "b", avatar: "/new-b.png", room_participation_paused: true },
    { agent_id: "a", avatar: "/new-a.png", room_participation_paused: false },
  ]);
  expect(contexts[0].member_agents[0].avatar).toBe("/old-b.png");
});
