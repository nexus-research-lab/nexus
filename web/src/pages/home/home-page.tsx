/**
 * 工作台（/app）
 */

import { useCallback, useEffect, useMemo, useState } from "react";

import { isMainAgent } from "@/config/options";
import { listRooms, subscribe_room_list_updates } from "@/lib/room-api";
import { useAgentStore } from "@/store/agent";
import { RoomAggregate } from "@/types/room";

import { HomeAsciiHero } from "./home-ascii-hero";
import { WorkspacePageFrame } from "@/shared/ui/workspace/workspace-page-frame";

export function HomePage() {
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const regular_agents = useMemo(
    () => agents.filter((agent) => !isMainAgent(agent.agent_id)),
    [agents],
  );
  const refresh_rooms = useCallback(() => {
    void listRooms(200).then(set_rooms).catch(() => {
    });
  }, []);

  useEffect(() => {
    void load_agents();
    refresh_rooms();
  }, [load_agents, refresh_rooms]);

  useEffect(() => subscribe_room_list_updates(refresh_rooms), [refresh_rooms]);

  return (
    <WorkspacePageFrame>
      <div className="flex min-h-0 flex-1 h-full">
        <HomeAsciiHero agent_count={regular_agents.length} room_count={rooms.length}/>
      </div>
    </WorkspacePageFrame>
  );
}
