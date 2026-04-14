"use client";

import { Check, Link2, Unplug } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBadge,
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTitle,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { ConnectorInfo } from "@/types/connector";

import { getConnectorColors, getConnectorLetter } from "./connector-icons";

interface ConnectorCardProps {
  connector: ConnectorInfo;
  busy?: boolean;
  on_select: () => void;
  on_connect?: () => void;
  on_disconnect?: () => void;
}

/** 连接器卡片 —— 与截图风格一致的三段式布局 */
export function ConnectorCard({
  connector,
  busy = false,
  on_select,
  on_connect,
  on_disconnect,
}: ConnectorCardProps) {
  const {
    title,
    description,
    icon,
    status,
    connection_state,
    is_configured,
    config_error,
    category,
    auth_type,
  } = connector;
  const colors = getConnectorColors(icon);
  const letter = getConnectorLetter(icon, title);
  const is_connected = connection_state === "connected";
  const is_coming_soon = status === "coming_soon";
  const auth_label =
    auth_type === "oauth2"
      ? "OAuth"
      : auth_type === "api_key"
        ? "API Key"
        : auth_type === "token"
          ? "Token"
          : "免授权";

  return (
    <WorkspaceCatalogCard
      class_name={cn(
        "group h-full",
        busy && "opacity-60",
      )}
      interactive
      onClick={on_select}
      size="catalog"
    >
      <WorkspaceCatalogHeader class_name="items-center">
        <WorkspaceIconFrame
          class_name={cn("h-10 w-10 shrink-0 text-sm font-bold", colors.bg, colors.text)}
          size="md"
        >
          {letter}
        </WorkspaceIconFrame>

        <div className="min-w-0 flex-1">
          <div className="min-w-0">
            <WorkspaceCatalogTitle class_name="min-w-0" size="sm" truncate>
              {title}
            </WorkspaceCatalogTitle>
            <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-(--text-soft)">
              <span>{category}</span>
              {is_coming_soon ? <span>即将推出</span> : null}
            </div>
          </div>
        </div>
      </WorkspaceCatalogHeader>

      <WorkspaceCatalogBody grow>
        <WorkspaceCatalogDescription min_height>
          {is_configured ? description : config_error || description}
        </WorkspaceCatalogDescription>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter>
        <div className="min-w-0 text-[11px] text-(--text-soft)">
          {auth_label}
        </div>

        <div className="flex shrink-0 items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
          {is_connected ? (
            <>
              <WorkspaceCatalogBadge tone="success">
                <Check className="h-3 w-3" />
                已连接
              </WorkspaceCatalogBadge>
              <WorkspaceCatalogAction
                disabled={busy}
                onClick={on_disconnect}
                size="sm"
                title="断开连接"
              >
                <Unplug className="h-3.5 w-3.5" />
              </WorkspaceCatalogAction>
            </>
          ) : !is_configured ? (
            <WorkspaceCatalogBadge tone="warning">
              未配置
            </WorkspaceCatalogBadge>
          ) : is_coming_soon ? (
            <WorkspaceCatalogBadge tone="neutral">
              即将推出
            </WorkspaceCatalogBadge>
          ) : (
            <>
              <WorkspaceCatalogBadge tone="info">
                <Link2 className="h-3 w-3" />
                可连接
              </WorkspaceCatalogBadge>
              <WorkspaceCatalogAction
                disabled={busy}
                onClick={on_connect}
                size="sm"
                title="授权连接"
              >
                <Link2 className="h-3.5 w-3.5" />
              </WorkspaceCatalogAction>
            </>
          )}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
