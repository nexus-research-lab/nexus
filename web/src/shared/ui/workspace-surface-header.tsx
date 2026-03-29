"use client";

import { LucideIcon } from "lucide-react";
import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceSurfaceHeaderTab<TTabKey extends string> {
  key: TTabKey;
  label: string;
  icon?: LucideIcon;
}

interface WorkspaceSurfaceHeaderProps<TTabKey extends string> {
  title: string;
  badge?: string;
  leading?: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  active_tab?: TTabKey;
  on_change_tab?: (tab: TTabKey) => void;
  class_name?: string;
}

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  title,
  badge,
  leading,
  subtitle,
  trailing,
  tabs = [],
  active_tab,
  on_change_tab,
  class_name,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  return (
    <div className={cn("z-10 overflow-hidden border-b workspace-divider bg-white/10 backdrop-blur-md", class_name)}>
      <div className="flex min-w-0 items-center justify-between gap-4 px-5 py-3 xl:px-6">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          {leading ? (
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/30 bg-white/16 text-slate-700 shadow-sm backdrop-blur-sm">
              {leading}
            </div>
          ) : null}

          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-2">
              <div className="truncate text-[17px] font-black tracking-[-0.04em] text-slate-950/88">
                {title}
              </div>
              {badge ? (
                <span className="inline-flex rounded-full border border-white/24 bg-white/16 px-2 py-0.5 text-[10px] font-semibold text-slate-600 backdrop-blur-sm">
                  {badge}
                </span>
              ) : null}
            </div>
            {subtitle ? (
              <div className="mt-1 flex items-center gap-2 text-[12px] text-slate-700/52">
                {subtitle}
              </div>
            ) : null}
          </div>
        </div>

        {trailing ? (
          <div className="ml-3 flex shrink-0 items-center gap-2">
            {trailing}
          </div>
        ) : null}
      </div>

      {tabs.length ? (
        <div className="flex items-center gap-1 px-5 pb-2 xl:px-6">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const is_active = active_tab === tab.key;
            return (
              <button
                key={tab.key}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
                  is_active
                    ? "workspace-chip text-slate-950 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                    : "text-slate-600 hover:bg-slate-100 hover:text-slate-950",
                )}
                onClick={() => on_change_tab?.(tab.key)}
                type="button"
              >
                {Icon ? <Icon className="h-3.5 w-3.5" /> : null}
                {tab.label}
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
