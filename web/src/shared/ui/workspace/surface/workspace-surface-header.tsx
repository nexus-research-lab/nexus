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
  titleTrailing?: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
  tabsLeading?: ReactNode;
  tabsTrailing?: ReactNode;
  activeTab?: TTabKey;
  onChangeTab?: (tab: TTabKey) => void;
}

interface WorkspaceSurfaceToolbarActionProps {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  tone?: "default" | "primary";
  ariaLabel?: string;
  className?: string;
  title?: string;
}

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  title,
  density = "default",
  leading,
  titleTrailing: titleTrailing,
  subtitle,
  trailing,
  tabs = [],
  tabsNavAnchor: tabsNavAnchor,
  tabsLeading: tabsLeading,
  tabsTrailing: tabsTrailing,
  activeTab: activeTab,
  onChangeTab: onChangeTab,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  const hasSecondaryRow = density === "compact" || tabs.length > 0 || Boolean(tabsLeading) || Boolean(tabsTrailing);
  const compactSubtitle = density === "compact" ? subtitle : null;
  const primarySubtitle = density === "compact" ? null : subtitle;
  const renderTabsNav = (className: string, ariaLabel: string) => (
    <UiUnderlineTabs
      activeValue={activeTab}
      ariaLabel={ariaLabel}
      className={className}
      density={density === "compact" ? "compact" : "default"}
      navAnchor={tabsNavAnchor}
      onChange={onChangeTab}
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
              {titleTrailing ? (
                <div className="min-w-0 shrink text-(--text-default)">{titleTrailing}</div>
              ) : null}
            </div>
            {primarySubtitle ? (
              <div className="mt-1 text-[12px] text-(--text-soft)">
                {primarySubtitle}
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

      {hasSecondaryRow ? (
        <div className={cn(
          "flex min-w-0",
          tabsLeading ? "px-3 xl:px-4" : "px-5 xl:px-6",
          density === "compact"
            ? cn(COMPACT_WORKSPACE_HEADER_SECONDARY_HEIGHT_CLASS, "items-center gap-3")
            : "items-end gap-4 pb-0.5",
        )}>
          {tabsLeading ? (
            <div className={cn("min-w-0 flex-1", density === "compact" && "self-start")}>{tabsLeading}</div>
          ) : tabs.length > 0 ? (
            renderTabsNav(
              cn(
                "soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 overflow-x-auto px-0.5",
                density === "compact" ? "items-center gap-3" : "items-center gap-4",
              ),
              "视图切换",
            )
          ) : compactSubtitle ? (
            <div className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-soft)">
              {compactSubtitle}
            </div>
          ) : (
            <div className="min-w-0 flex-1" />
          )}

          {tabsLeading && tabs.length > 0 ? (
            <>
              <div className="hidden h-5 w-px shrink-0 bg-(--divider-subtle-color) sm:block" />
              {renderTabsNav(
                cn(
                  "soft-scrollbar scrollbar-hide hidden min-w-0 shrink-0 overflow-x-auto sm:flex",
                  density === "compact" ? "items-center gap-3" : "items-center gap-4",
                ),
                "固定视图切换",
              )}
            </>
          ) : null}

          {tabsTrailing ? (
            <div className="shrink-0">
              {tabsTrailing}
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
  ariaLabel: ariaLabel,
  className: className,
  title,
}: WorkspaceSurfaceToolbarActionProps) {
  return (
    <button
      aria-label={ariaLabel}
      className={cn(
        "inline-flex items-center gap-1.5 text-[11px] font-semibold transition duration-(--motion-duration-fast) ease-out disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        tone === "default" && "text-(--text-default) hover:text-(--text-strong)",
        tone === "primary" && "text-(--primary) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]",
        className,
      )}
      disabled={disabled}
      onClick={onClick}
      title={title}
      type="button"
    >
      {children}
    </button>
  );
}
