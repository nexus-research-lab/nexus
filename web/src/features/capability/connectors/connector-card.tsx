"use client";

import { Check, Link2, Unplug } from "lucide-react";

import { cn } from "@/lib/utils";
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
    <article
      className={cn(
        "group relative flex cursor-pointer workspace-card flex-col rounded-[22px] px-5 py-4 transition-all hover:border-white/36 hover:bg-white/40",
        is_coming_soon && "opacity-70",
      )}
      onClick={on_select}
    >
      {/* 顶部：图标 + 标题 + 状态 */}
      <div className="flex items-start gap-3">
        {/* 品牌图标 */}
        <div
          className={cn(
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-[14px] border border-white/50 text-sm font-bold",
            colors.bg,
            colors.text,
          )}
        >
          {letter}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-[14px] font-semibold text-slate-900">
              {title}
            </h3>
          </div>
          <p className="mt-0.5 line-clamp-2 text-[12px] leading-relaxed text-slate-500">
            {is_configured ? description : config_error || description}
          </p>
        </div>

        {/* 右上角状态 */}
        <div className="shrink-0">
          {is_connected ? (
            <span className="flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-600">
              <Check className="h-3 w-3" />
              已连接
            </span>
          ) : is_coming_soon ? (
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-400">
              即将推出
            </span>
          ) : !is_configured ? (
            <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-600">
              待配置
            </span>
          ) : (
            <span className="flex items-center gap-1 rounded-full bg-slate-50 px-2 py-0.5 text-[11px] font-medium text-slate-400">
              未连接
            </span>
          )}
        </div>
      </div>

      {/* 底部操作 —— 可用的连接器才显示 */}
      {!is_coming_soon && (
        <div className="mt-3 flex items-center justify-end" onClick={(e) => e.stopPropagation()}>
          {is_connected ? (
            <button
              className="flex items-center gap-1 rounded-full px-3 py-1 text-[12px] font-medium text-slate-500 transition-colors hover:bg-red-50 hover:text-red-600"
              disabled={busy}
              onClick={on_disconnect}
              type="button"
            >
              <Unplug className="h-3 w-3" />
              断开
            </button>
          ) : (
            <button
              className="flex items-center gap-1 rounded-full bg-sky-500 px-3 py-1.5 text-[12px] font-medium text-white shadow-sm transition-all hover:bg-sky-600"
              disabled={busy || !is_configured}
              onClick={on_connect}
              type="button"
            >
              <Link2 className="h-3 w-3" />
              {is_configured ? "连接" : "未配置"}
            </button>
          )}
        </div>
      )}
    </article>
  );
}
