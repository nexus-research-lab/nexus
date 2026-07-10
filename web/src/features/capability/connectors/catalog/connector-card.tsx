"use client";

import { Check, Clock3, KeyRound, Loader2, Plus, Settings2 } from "lucide-react";
import { type KeyboardEvent, type MouseEvent } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import { UiIconButton } from "@/shared/ui/button";
import type { ConnectorInfo } from "@/types/capability/connector";

import { isDirectCredentialAuth } from "../auth/connector-auth";
import { ConnectorIcon } from "../connector-icon";
import { getConnectorCategoryLabel } from "./connectors-categories";

interface ConnectorCardProps {
  connector: ConnectorInfo;
  busy?: boolean;
  onSelect: () => void;
  onConnect?: () => void;
}

/** 连接器行 —— 学习 Codex 插件目录的轻量列表结构。 */
export function ConnectorCard({
  connector,
  busy = false,
  onSelect,
  onConnect,
}: ConnectorCardProps) {
  const { t } = useI18n();
  const {
    title,
    description,
    icon,
    status,
    connection_state: connectionState,
    is_configured: isConfigured,
    category,
    oauth_client_config_required: oauthClientConfigRequired,
  } = connector;
  const isConnected = connectionState === "connected";
  const isComingSoon = status === "coming_soon";
  const requiresDirectCredential = isDirectCredentialAuth(connector.auth_type);
  const shouldConfigure = !isConfigured && oauthClientConfigRequired;
  const canConnect = !busy && !isConnected && !isComingSoon && isConfigured;

  const handleActionClick = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (canConnect && !requiresDirectCredential) {
      onConnect?.();
      return;
    }
    onSelect();
  };

  const handleRowKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) {
      return;
    }
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    onSelect();
  };

  return (
    <div
      className={cn(
        "group flex min-h-[64px] w-full items-center gap-3 rounded-[14px] px-2 py-1.5 text-left outline-none transition-[background-color]",
        "hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_64%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]",
        busy && "opacity-65",
      )}
      onClick={onSelect}
      onKeyDown={handleRowKeyDown}
      role="button"
      tabIndex={0}
    >
      <ConnectorIcon icon={icon} title={title} />

      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {title}
          </span>
          {isComingSoon ? (
            <UiBadge size="xs">
              即将推出
            </UiBadge>
          ) : shouldConfigure ? (
            <UiBadge size="xs" tone="warning">
              待配置
            </UiBadge>
          ) : null}
        </span>
        <span className="mt-0.5 block truncate text-[13px] leading-5 text-(--text-muted)">
          {description}
        </span>
        <span className="mt-0.5 block text-[11px] leading-4 text-(--text-soft)">
          {getConnectorCategoryLabel(category, t)}
        </span>
      </span>

      <span className="flex h-9 w-9 shrink-0 items-center justify-center">
        {busy ? (
          <Loader2 className="h-4 w-4 animate-spin text-(--icon-default)" />
        ) : isConnected ? (
          <Check className="h-4 w-4 text-(--icon-muted)" />
        ) : isComingSoon ? (
          <Clock3 className="h-4 w-4 text-(--icon-muted)" />
        ) : (
          <UiIconButton
            aria-label={shouldConfigure || requiresDirectCredential ? `配置 ${title}` : `连接 ${title}`}
            onClick={handleActionClick}
            size="md"
            type="button"
          >
            {shouldConfigure ? (
              <Settings2 className="h-4 w-4" />
            ) : requiresDirectCredential ? (
              <KeyRound className="h-4 w-4" />
            ) : (
              <Plus className="h-4 w-4" />
            )}
          </UiIconButton>
        )}
      </span>
    </div>
  );
}
