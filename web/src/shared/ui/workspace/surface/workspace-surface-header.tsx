"use client";

import { LucideIcon } from "lucide-react";
import { ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  COMPACT_WORKSPACE_HEADER_PRIMARY_HEIGHT_CLASS,
  COMPACT_WORKSPACE_HEADER_SECONDARY_HEIGHT_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";

export { WorkspaceTaskStrip } from "./workspace-task-strip";

const SURFACE_HEADER_CLASS_NAME =
  "border-b border-(--divider-subtle-color) bg-transparent";

interface WorkspaceSurfaceHeaderTab<TTabKey extends string> {
  key: TTabKey;
  label: string;
  icon?: LucideIcon;
}

interface WorkspaceSurfaceHeaderProps<TTabKey extends string> {
  title: string;
  badge?: string;
  density?: "default" | "compact";
  leading?: ReactNode;
  title_trailing?: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabs_trailing?: ReactNode;
  active_tab?: TTabKey;
  on_change_tab?: (tab: TTabKey) => void;
}

interface WorkspaceSurfaceToolbarActionProps {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  tone?: "default" | "primary";
}

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  title,
  density = "default",
  leading,
  title_trailing,
  subtitle,
  trailing,
  tabs = [],
  tabs_trailing,
  active_tab,
  on_change_tab,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  return (
    <div className={SURFACE_HEADER_CLASS_NAME} data-density={density}>
      <div className={cn(
        "flex min-w-0 items-center justify-between px-5 xl:px-6",
        density === "compact" ? cn(COMPACT_WORKSPACE_HEADER_PRIMARY_HEIGHT_CLASS, "gap-3") : "h-[72px] gap-4",
      )}>
        <div className={cn("flex min-w-0 flex-1 items-center", density === "compact" ? "gap-2.5" : "gap-3")}>
          {leading ? (
            <div
              className={cn(
                "flex shrink-0 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)",
                density === "compact" ? "h-8 w-8" : "h-10 w-10",
              )}
            >
              {leading}
            </div>
          ) : null}

          <div className="min-w-0 flex-1">
            <div className={cn("flex min-w-0 flex-wrap items-center", density === "compact" ? "gap-x-1.5 gap-y-0.5" : "gap-x-2 gap-y-1")}>
              <div className={cn(
                "truncate font-black tracking-[-0.045em] text-(--text-strong)",
                density === "compact" ? "text-[20px]" : "text-[21px]",
              )}>
                {title}
              </div>
              {title_trailing ? (
                <div className="min-w-0 shrink text-(--text-default)">{title_trailing}</div>
              ) : null}
            </div>
            {subtitle ? (
              <div className="mt-1 text-[12px] text-(--text-soft)">
                {subtitle}
              </div>
            ) : null}
          </div>
        </div>

        {trailing ? (
          <div className={cn("ml-3 flex shrink-0 flex-wrap items-center justify-end", density === "compact" ? "gap-1.5" : "gap-2")}>
            {trailing}
          </div>
        ) : null}
      </div>

      {tabs.length || tabs_trailing ? (
        <div className={cn(
          "flex min-w-0 px-5 xl:px-6",
          density === "compact"
            ? cn(COMPACT_WORKSPACE_HEADER_SECONDARY_HEIGHT_CLASS, "items-center gap-3")
            : "items-end gap-4 pb-0.5",
        )}>
          <nav
            aria-label="视图切换"
            className={cn(
              "soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 overflow-x-auto px-0.5",
              density === "compact" ? "items-center gap-3" : "items-center gap-4",
            )}
          >
            {tabs.map((tab) => {
              const Icon = tab.icon;
              const is_active = active_tab === tab.key;
              return (
                <button
                  aria-pressed={is_active}
                  aria-current={is_active ? "page" : undefined}
                  key={tab.key}
                  className={cn(
                    "inline-flex h-9 shrink-0 items-center gap-1.5 border-b-2 border-transparent px-0 py-0 text-[11px] font-semibold transition-[color,border-color] duration-(--motion-duration-fast) ease-out",
                    is_active
                      ? "border-(--surface-interactive-active-border) text-(--text-strong)"
                      : "text-(--text-default) hover:text-(--text-strong)",
                    density === "compact" && "h-8 text-[10.5px]",
                  )}
                  onClick={() => on_change_tab?.(tab.key)}
                  type="button"
                >
                  {Icon ? <Icon className="h-3.5 w-3.5" /> : null}
                  {tab.label}
                </button>
              );
            })}
          </nav>
          {tabs_trailing ? (
            <div className="shrink-0">
              {tabs_trailing}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export function WorkspaceSurfaceToolbarAction({
  children,
  onClick,
  disabled = false,
  tone = "default",
}: WorkspaceSurfaceToolbarActionProps) {
  return (
    <button
      className={cn(
        "inline-flex items-center gap-1.5 text-[11px] font-semibold transition duration-(--motion-duration-fast) ease-out disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        tone === "default" && "text-(--text-default) hover:text-(--text-strong)",
        tone === "primary" && "text-(--primary) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]",
      )}
      disabled={disabled}
      onClick={onClick}
      type="button"
    >
      {children}
    </button>
  );
}
