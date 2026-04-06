"use client";

import { Check, Link2, Unplug } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogBadge,
  WorkspaceCatalogCard,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
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
  const { title, description, icon, status, connection_state, is_configured, config_error } = connector;
  const colors = getConnectorColors(icon);
  const letter = getConnectorLetter(icon, title);
  const is_connected = connection_state === "connected";
  const is_coming_soon = status === "coming_soon";

  return (
    <WorkspaceCatalogCard
      class_name={cn(
        "group cursor-pointer rounded-[22px] px-5 py-4",
        is_coming_soon && "opacity-70",
      )}
      onClick={on_select}
    >
      {/* 顶部：图标 + 标题 + 状态 */}
      <div className="flex items-start gap-3">
        {/* 品牌图标 */}
        <WorkspaceIconFrame
          class_name={cn("h-10 w-10 shrink-0 text-sm font-bold", colors.bg, colors.text)}
          size="md"
        >
          {letter}
        </WorkspaceIconFrame>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-[15px] font-semibold text-slate-950/98">
              {title}
            </h3>
          </div>
          <p className="mt-0.5 line-clamp-2 text-[12px] leading-[1.55] text-slate-700/78">
            {is_configured ? description : config_error || description}
          </p>
        </div>

        {/* 右上角状态 */}
        <div className="shrink-0">
          {is_connected ? (
            <WorkspaceCatalogBadge tone="success">
              <Check className="h-3 w-3" />
              已连接
            </WorkspaceCatalogBadge>
          ) : is_coming_soon ? (
            <WorkspaceCatalogBadge tone="neutral">
              即将推出
            </WorkspaceCatalogBadge>
          ) : !is_configured ? (
            <WorkspaceCatalogBadge tone="warning">
              待配置
            </WorkspaceCatalogBadge>
          ) : (
            <WorkspaceCatalogBadge tone="neutral">
              未连接
            </WorkspaceCatalogBadge>
          )}
        </div>
      </div>

      {/* 底部操作 —— 可用的连接器才显示 */}
      {!is_coming_soon && (
        <div className="mt-3 flex items-center justify-end gap-2" onClick={(e) => e.stopPropagation()}>
          {is_connected ? (
            <WorkspacePillButton
              disabled={busy}
              density="compact"
              onClick={on_disconnect}
              size="sm"
              variant="outlined"
            >
              <Unplug className="h-3 w-3" />
              断开
            </WorkspacePillButton>
          ) : (
            <WorkspacePillButton
              disabled={busy || !is_configured}
              density="compact"
              onClick={on_connect}
              size="sm"
              variant={is_configured ? "primary" : "outlined"}
            >
              <Link2 className="h-3 w-3" />
              {is_configured ? "连接" : "未配置"}
            </WorkspacePillButton>
          )}
        </div>
      )}
    </WorkspaceCatalogCard>
  );
}
