import { Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";

import type { ConnectorController } from "@/hooks/capability/use-connector-controller";

const CONNECTOR_CATEGORIES = [
  { key: "all", label: "全部" },
  { key: "productivity", label: "效率工具" },
  { key: "social", label: "社交媒体" },
  { key: "ecommerce", label: "电商平台" },
  { key: "development", label: "开发工具" },
  { key: "business", label: "企业管理" },
  { key: "marketing", label: "营销分析" },
  { key: "automation", label: "自动化" },
];

interface ConnectorsHeaderProps {
  ctrl: ConnectorController;
}

export function ConnectorsHeader({ ctrl }: ConnectorsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      active_tab={ctrl.active_category}
      badge={t("capability.connected_badge", { count: ctrl.connected_count })}
      density="compact"
      leading={<Link2 className="h-4 w-4" />}
      tabs={CONNECTOR_CATEGORIES}
      title={t("capability.connectors_title")}
      on_change_tab={ctrl.set_active_category}
    />
  );
}
