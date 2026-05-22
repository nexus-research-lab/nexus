import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";

import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsHeaderProps {
  ctrl: ConnectorDirectoryController;
}

export function ConnectorsHeader({ ctrl }: ConnectorsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.connected_badge", { count: ctrl.connected_count })}
      leading={<Link2 className="h-4 w-4" />}
      title={t("capability.connectors_title")}
    />
  );
}
