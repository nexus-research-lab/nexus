/**
 * INPUT: Connector 身份、短摘要、状态动作与选择命令。
 * OUTPUT: 不重复分类元数据的紧凑 Connector 目录条目。
 * POS: Connector 目录卡片纯视图。
 */
"use client";

import { Clock3, KeyRound, Loader2, Plus, Settings2, Unplug } from "lucide-react";
import { type MouseEvent } from "react";

import { CAPABILITY_DIRECTORY_ROW_CLASS_NAME } from "@/features/capability/shared/capability-page-layout";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
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

  const handleActionClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
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
      className={cn(CAPABILITY_DIRECTORY_ROW_CLASS_NAME, busy && "opacity-65")}
      description={connector.description}
      leading={<ConnectorIcon icon={connector.icon} title={connector.title} />}
      meta={<ConnectorCardBadge badge={model.badge} />}
      onClick={onSelect}
      right={(
        <span className="flex h-9 w-9 shrink-0 items-center justify-center">
          <ConnectorCardTrailing
            model={model.trailing}
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
  if (!badge) return null;
  return <UiBadge size="xs" tone={badge.tone}>{badge.label}</UiBadge>;
}

const ACTION_ICON = {
  connect: Plus,
  credential: KeyRound,
  disconnect: Unplug,
  "oauth-client": Settings2,
} as const;

const STATIC_TRAILING = {
  busy: () => <Loader2 className="h-4 w-4 animate-spin text-(--icon-default)" />,
  "coming-soon": () => <Clock3 className="h-4 w-4 text-(--icon-muted)" />,
} as const;

function ConnectorCardTrailing({
  model,
  onAction,
}: {
  model: ConnectorCardTrailingModel;
  onAction: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  if (model.kind !== "action") return STATIC_TRAILING[model.kind]();
  const Icon = ACTION_ICON[model.icon];
  return (
    <UiIconButton
      aria-label={model.ariaLabel}
      onClick={onAction}
      size="md"
      type="button"
    >
      <Icon className="h-4 w-4" />
    </UiIconButton>
  );
}
