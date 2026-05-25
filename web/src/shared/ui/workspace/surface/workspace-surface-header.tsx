"use client";

import { type LucideIcon } from "lucide-react";
import { type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { UiUnderlineTabs } from "@/shared/ui/tabs";
import {
  COMPACT_WORKSPACE_HEADER_PRIMARY_HEIGHT_CLASS,
  COMPACT_WORKSPACE_HEADER_SECONDARY_HEIGHT_CLASS,
  COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";

export { WorkspaceTaskStrip } from "./workspace-task-strip";

const SURFACE_HEADER_CLASS_NAME =
  "border-b border-(--divider-subtle-color) bg-transparent";

interface WorkspaceSurfaceHeaderTab<TTabKey extends string> {
  key: TTabKey;
  label: string;
  icon?: LucideIcon;
  anchor?: string;
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
  tabs_nav_anchor?: string;
  tabs_leading?: ReactNode;
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
  tabs_nav_anchor,
  tabs_leading,
  tabs_trailing,
  active_tab,
  on_change_tab,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  const has_secondary_row = density === "compact" || tabs.length > 0 || Boolean(tabs_leading) || Boolean(tabs_trailing);
  const compact_subtitle = density === "compact" ? subtitle : null;
  const primary_subtitle = density === "compact" ? null : subtitle;
  const render_tabs_nav = (class_name: string, aria_label: string) => (
    <UiUnderlineTabs
      active_value={active_tab}
      aria_label={aria_label}
      class_name={class_name}
      density={density === "compact" ? "compact" : "default"}
      nav_anchor={tabs_nav_anchor}
      on_change={on_change_tab}
      options={tabs.map((tab) => ({
        anchor: tab.anchor,
        icon: tab.icon,
        label: tab.label,
        value: tab.key,
      }))}
    />
  );

  return (
    <div className={SURFACE_HEADER_CLASS_NAME} data-density={density}>
      <div className={cn(
        "flex min-w-0 items-center justify-between px-5 xl:px-6",
        density === "compact" ? cn(COMPACT_WORKSPACE_HEADER_PRIMARY_HEIGHT_CLASS, "gap-3") : cn(COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS, "gap-3"),
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
            {primary_subtitle ? (
              <div className="mt-1 text-[12px] text-(--text-soft)">
                {primary_subtitle}
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

      {has_secondary_row ? (
        <div className={cn(
          "flex min-w-0",
          tabs_leading ? "px-3 xl:px-4" : "px-5 xl:px-6",
          density === "compact"
            ? cn(COMPACT_WORKSPACE_HEADER_SECONDARY_HEIGHT_CLASS, "items-center gap-3")
            : "items-end gap-4 pb-0.5",
        )}>
          {tabs_leading ? (
            <div className={cn("min-w-0 flex-1", density === "compact" && "self-start")}>{tabs_leading}</div>
          ) : tabs.length > 0 ? (
            render_tabs_nav(
              cn(
                "soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 overflow-x-auto px-0.5",
                density === "compact" ? "items-center gap-3" : "items-center gap-4",
              ),
              "视图切换",
            )
          ) : compact_subtitle ? (
            <div className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-soft)">
              {compact_subtitle}
            </div>
          ) : (
            <div className="min-w-0 flex-1" />
          )}

          {tabs_leading && tabs.length > 0 ? (
            <>
              <div className="hidden h-5 w-px shrink-0 bg-(--divider-subtle-color) sm:block" />
              {render_tabs_nav(
                cn(
                  "soft-scrollbar scrollbar-hide hidden min-w-0 shrink-0 overflow-x-auto sm:flex",
                  density === "compact" ? "items-center gap-3" : "items-center gap-4",
                ),
                "固定视图切换",
              )}
            </>
          ) : null}

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
