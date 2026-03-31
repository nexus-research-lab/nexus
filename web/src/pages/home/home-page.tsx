/**
 * 工作台（/app）
 */

import { useEffect, useState } from "react";

import { listRooms } from "@/lib/room-api";
import { useAgentStore } from "@/store/agent";
import { RoomAggregate } from "@/types/room";

import { HomeAsciiHero } from "./home-ascii-hero";
import { WorkspacePageFrame } from "@/shared/ui/workspace/workspace-page-frame";

export function HomePage() {
  const agents = useAgentStore((s) => s.agents);
  const load_agents = useAgentStore((s) => s.load_agents_from_server);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);

  useEffect(() => {
    void load_agents();
    void listRooms(200).then(set_rooms).catch(() => {
    });
  }, [load_agents]);

  return (
    <WorkspacePageFrame>
      <div className="flex min-h-0 flex-1 h-full">
        <HomeAsciiHero agent_count={agents.length} room_count={rooms.length}/>
      </div>
    </WorkspacePageFrame>
  );
}
