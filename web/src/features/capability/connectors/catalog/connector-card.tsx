/**
 * INPUT: Connector 身份、短摘要、状态动作与选择命令。
 * OUTPUT: 公共 outlined 条目、本地化状态及由 ListAction 隔离的目录次动作。
 * POS: Connector 目录卡片纯视图。
 */
"use client";

import { Clock3, KeyRound, Loader2, Plus, Settings2, Unplug } from "lucide-react";

import { CAPABILITY_DIRECTORY_ROW_CLASS_NAME } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { ConnectorInfo } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import {
  buildConnectorCardModel,
  type ConnectorCardBadgeModel,
  type ConnectorCardTrailingModel,
} from "./connector-card-model";

interface ConnectorCardProps {
  busy?: boolean;
  connector: ConnectorInfo;
  onConnect?: () => void;
  onDisconnect?: () => void;
  onSelect: () => void;
}

export function ConnectorCard({
  busy = false,
  connector,
  onConnect,
  onDisconnect,
  onSelect,
}: ConnectorCardProps) {
  const model = buildConnectorCardModel(connector, busy);

  const handleActionClick = () => {
    if (model.trailing.kind !== "action") return;
    if (model.trailing.action === "connect") {
      onConnect?.();
      return;
    }
    if (model.trailing.action === "disconnect") {
      onDisconnect?.();
      return;
    }
    onSelect();
  };

  return (
    <UiListRow
      variant="outlined"
      className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
      description={connector.description}
      leading={<ConnectorIcon icon={connector.icon} title={connector.title} />}
      meta={<ConnectorCardBadge badge={model.badge} />}
      muted={busy}
      onClick={onSelect}
      right={(
        <span className="flex h-9 w-9 shrink-0 items-center justify-center">
          <ConnectorCardTrailing
            model={model.trailing}
            name={connector.title}
            onAction={handleActionClick}
          />
        </span>
      )}
      title={connector.title}
    />
  );
}

function ConnectorCardBadge({
  badge,
}: {
  badge: ConnectorCardBadgeModel | null;
}) {
  const { t } = useI18n();
  if (!badge) return null;
  return <UiBadge size="xs" tone={badge.tone}>{t(badge.labelKey)}</UiBadge>;
}

const ACTION_ICON = {
  connect: Plus,
  credential: KeyRound,
  disconnect: Unplug,
  "oauth-client": Settings2,
} as const;

const STATIC_TRAILING = {
  busy: () => <Loader2 className={getUiSpinnerClassName({ size: "md" })} />,
  "coming-soon": () => <Clock3 className="h-4 w-4 text-(--icon-muted)" />,
} as const;

function ConnectorCardTrailing({
  model,
  name,
  onAction,
}: {
  model: ConnectorCardTrailingModel;
  name: string;
  onAction: () => void;
}) {
  const { t } = useI18n();
  if (model.kind !== "action") return STATIC_TRAILING[model.kind]();
  const Icon = ACTION_ICON[model.icon];
  return (
    <UiListActionButton
      aria-label={t(model.ariaLabelKey, { name })}
      onClick={onAction}
      size="md"
      stopPropagation
      visibility="visible"
    >
      <Icon className="h-4 w-4" />
    </UiListActionButton>
  );
}
