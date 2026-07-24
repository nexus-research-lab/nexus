import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";

export function ConnectorsHeader() {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      leading={<Link2 className="h-4 w-4" />}
      subtitle={t("capability.connectors_subtitle")}
      title={t("capability.connectors")}
    />
  );
}
