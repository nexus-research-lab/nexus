"use client";

import { Check, Clock3, Loader2, Plus, Settings2 } from "lucide-react";
import { type KeyboardEvent, type MouseEvent } from "react";

import { cn } from "@/lib/utils";
import { UiBadge } from "@/shared/ui/badge";
import { UiIconButton } from "@/shared/ui/button";
import { ConnectorInfo } from "@/types/capability/connector";

import { get_connector_category_label } from "./connectors-categories";
import { get_connector_colors, get_connector_letter } from "./connector-icons";

interface ConnectorCardProps {
  connector: ConnectorInfo;
  busy?: boolean;
  on_select: () => void;
  on_connect?: () => void;
}

/** 连接器行 —— 学习 Codex 插件目录的轻量列表结构。 */
export function ConnectorCard({
  connector,
  busy = false,
  on_select,
  on_connect,
}: ConnectorCardProps) {
  const {
    title,
    description,
    icon,
    status,
    connection_state,
    is_configured,
    category,
    oauth_client_config_required,
  } = connector;
  const colors = get_connector_colors(icon);
  const letter = get_connector_letter(icon, title);
  const is_connected = connection_state === "connected";
  const is_coming_soon = status === "coming_soon";
  const should_configure = !is_configured && oauth_client_config_required;
  const can_connect = !busy && !is_connected && !is_coming_soon && is_configured;

  const handle_action_click = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (can_connect) {
      on_connect?.();
      return;
    }
    on_select();
  };

  const handle_row_key_down = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) {
      return;
    }
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    on_select();
  };

  return (
    <div
      className={cn(
        "group flex min-h-[64px] w-full items-center gap-3 rounded-[14px] px-2 py-1.5 text-left outline-none transition-[background-color]",
        "hover:bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_64%,transparent)] focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]",
        busy && "opacity-65",
      )}
      onClick={on_select}
      onKeyDown={handle_row_key_down}
      role="button"
      tabIndex={0}
    >
      <span
        className={cn(
          "flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] text-[13px] font-semibold shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
          colors.bg,
          colors.text,
        )}
      >
        {letter}
      </span>

      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {title}
          </span>
          {is_coming_soon ? (
            <UiBadge size="xs">
              即将推出
            </UiBadge>
          ) : should_configure ? (
            <UiBadge size="xs" tone="warning">
              待配置
            </UiBadge>
          ) : null}
        </span>
        <span className="mt-0.5 block truncate text-[13px] leading-5 text-(--text-muted)">
          {description}
        </span>
        <span className="mt-0.5 block text-[11px] leading-4 text-(--text-soft)">
          {get_connector_category_label(category)}
        </span>
      </span>

      <span className="flex h-9 w-9 shrink-0 items-center justify-center">
        {busy ? (
          <Loader2 className="h-4 w-4 animate-spin text-(--icon-default)" />
        ) : is_connected ? (
          <Check className="h-4 w-4 text-(--icon-muted)" />
        ) : is_coming_soon ? (
          <Clock3 className="h-4 w-4 text-(--icon-muted)" />
        ) : (
          <UiIconButton
            aria-label={should_configure ? `配置 ${title}` : `连接 ${title}`}
            onClick={handle_action_click}
            size="md"
            type="button"
          >
            {should_configure ? (
              <Settings2 className="h-4 w-4" />
            ) : (
              <Plus className="h-4 w-4" />
            )}
          </UiIconButton>
        )}
      </span>
    </div>
  );
}
