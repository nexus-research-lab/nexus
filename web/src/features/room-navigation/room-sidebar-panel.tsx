import { ComponentProps } from "react";

import { WorkspaceSidebar } from "@/components/workspace/workspace-sidebar";

export type RoomSidebarPanelProps = ComponentProps<typeof WorkspaceSidebar>;

export function RoomSidebarPanel(props: RoomSidebarPanelProps) {
  return <WorkspaceSidebar {...props} />;
}
