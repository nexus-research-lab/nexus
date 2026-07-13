import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";

interface ConnectorsHeaderProps {
  connectedCount: number;
}

export function ConnectorsHeader({ connectedCount }: ConnectorsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.connected_badge", { count: connectedCount })}
      leading={<Link2 className="h-4 w-4" />}
      subtitle={t("capability.connectors_subtitle")}
      title={t("capability.connectors")}
    />
  );
}
