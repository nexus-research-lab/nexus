"use client";

import { Link2, Unplug } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTag,
  WorkspaceCatalogTitle,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { ConnectorInfo } from "@/types/capability/connector";

import { get_connector_colors, get_connector_letter } from "./connector-icons";

interface ConnectorCardProps {
  connector: ConnectorInfo;
  busy?: boolean;
  on_select: () => void;
  on_connect?: () => void;
  on_disconnect?: () => void;
}

function ConnectorStatePill({
  children,
  tone = "neutral",
}: {
  children: string;
  tone?: "neutral" | "success" | "warning" | "info";
}) {
  const tone_class_name =
    tone === "warning"
      ? "border-amber-200/80 bg-amber-50/88 text-amber-700"
      : tone === "success"
        ? "border-emerald-200/80 bg-emerald-50/90 text-emerald-700"
        : tone === "info"
          ? "border-sky-200/80 bg-sky-50/90 text-sky-700"
          : "border-(--surface-panel-subtle-border) bg-(--surface-panel-subtle-background) text-(--text-soft)";

  return (
    <span
      className={cn(
        "inline-flex h-6 items-center rounded-full border px-2.5 text-[11px] font-medium leading-none tracking-[0.01em]",
        tone_class_name,
      )}
    >
      {children}
    </span>
  );
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
  const colors = get_connector_colors(icon);
  const letter = get_connector_letter(icon, title);
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
  const state_label = is_connected ? "已连接" : !is_configured ? "未配置" : is_coming_soon ? "即将推出" : "可连接";
  const state_tone = is_connected ? "success" : !is_configured ? "warning" : !is_coming_soon ? "info" : "neutral";

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
      <WorkspaceCatalogHeader class_name="items-center gap-3.5">
        <WorkspaceIconFrame
          class_name={cn("shrink-0 text-[13px] font-semibold", colors.bg, colors.text)}
          size="sm"
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
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <WorkspaceCatalogTag class_name="px-2.5 text-[10px] text-(--text-soft)">
            {auth_label}
          </WorkspaceCatalogTag>
        </div>

        <div className="flex shrink-0 items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
          <ConnectorStatePill tone={state_tone}>
            {state_label}
          </ConnectorStatePill>
          {is_connected ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_disconnect}
              size="sm"
              title="断开连接"
            >
              <Unplug className="h-3 w-3" />
            </WorkspaceCatalogAction>
          ) : !is_configured || is_coming_soon ? null : (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_connect}
              size="sm"
              title="授权连接"
            >
              <Link2 className="h-3 w-3" />
            </WorkspaceCatalogAction>
          )}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
