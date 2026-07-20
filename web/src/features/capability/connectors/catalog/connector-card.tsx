"use client";

import { Check, Clock3, KeyRound, Loader2, Plus, Settings2 } from "lucide-react";
import { type KeyboardEvent, type MouseEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import type { ConnectorInfo } from "@/types/capability/connector";

import { ConnectorIcon } from "../connector-icon";
import {
  buildConnectorCardModel,
  type ConnectorCardBadgeModel,
  type ConnectorCardTrailingModel,
} from "./connector-card-model";
import { getConnectorCategoryLabel } from "./connectors-categories";

interface ConnectorCardProps {
  busy?: boolean;
  connector: ConnectorInfo;
  onConnect?: () => void;
  onSelect: () => void;
}

export function ConnectorCard({
  busy = false,
  connector,
  onConnect,
  onSelect,
}: ConnectorCardProps) {
  const { t } = useI18n();
  const model = buildConnectorCardModel(connector, busy);

  const handleActionClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (model.trailing.kind !== "action") return;
    if (model.trailing.action === "connect") {
      onConnect?.();
      return;
    }
    onSelect();
  };

  const handleRowKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    onSelect();
  };

  return (
    <div
      className={cn(
        "group flex min-h-[64px] w-full items-center gap-2.5 rounded-[8px] px-2 py-1 text-left outline-none transition-[background-color]",
        "hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_64%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]",
        busy && "opacity-65",
      )}
      onClick={onSelect}
      onKeyDown={handleRowKeyDown}
      role="button"
      tabIndex={0}
    >
      <ConnectorIcon icon={connector.icon} title={connector.title} />
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[14px] font-medium text-(--text-strong)">
            {connector.title}
          </span>
          <ConnectorCardBadge badge={model.badge} />
        </span>
        <span className="mt-0.5 block truncate text-[12px] leading-[1.125rem] text-(--text-muted)">
          {connector.description}
        </span>
        <span className="mt-0.5 block text-[10px] leading-4 text-(--text-soft)">
          {getConnectorCategoryLabel(connector.category, t)}
        </span>
      </span>
      <span className="flex h-9 w-9 shrink-0 items-center justify-center">
        <ConnectorCardTrailing
          model={model.trailing}
          onAction={handleActionClick}
        />
      </span>
    </div>
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
  "oauth-client": Settings2,
} as const;

const STATIC_TRAILING = {
  busy: () => <Loader2 className="h-4 w-4 animate-spin text-(--icon-default)" />,
  connected: () => <Check className="h-4 w-4 text-(--icon-muted)" />,
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
