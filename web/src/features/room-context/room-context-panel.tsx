import { ComponentProps } from "react";

import { AgentInspector } from "@/components/workspace/agent-inspector";

export type RoomContextPanelProps = ComponentProps<typeof AgentInspector>;

export function RoomContextPanel(props: RoomContextPanelProps) {
  return <AgentInspector {...props} />;
}
