import { Link2 } from "lucide-react";

import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";

import type { ConnectorController } from "@/hooks/use-connector-controller";

interface ConnectorsHeaderProps {
  ctrl: ConnectorController;
}

export function ConnectorsHeader({ ctrl }: ConnectorsHeaderProps) {
  return (
    <WorkspaceSurfaceHeader
      badge={`已连接 ${ctrl.connected_count}`}
      leading={<Link2 className="h-4 w-4" />}
      subtitle="授权第三方平台，让 Agent 代表你访问数据"
      title="应用授权"
    />
  );
}
