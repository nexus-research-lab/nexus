"use client";

import { type LucideIcon } from "lucide-react";
import { type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { UiUnderlineTabs } from "@/shared/ui/tabs";
import {
  COMPACT_WORKSPACE_HEADER_SINGLE_ROW_HEIGHT_CLASS,
  WORKSPACE_HEADER_DEFAULT_HEIGHT_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";
import "./workspace-surface-header.css";

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
  onDismissActiveTab?: (tab: TTabKey) => void;
  dismissActiveTabLabel?: string;
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
  badge,
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
  onDismissActiveTab: onDismissActiveTab,
  dismissActiveTabLabel: dismissActiveTabLabel,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  const usesSingleRow = density === "compact";
  const hasSecondaryRow = !usesSingleRow && (tabs.length > 0 || Boolean(tabsLeading) || Boolean(tabsTrailing));
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
      onDismissActive={onDismissActiveTab}
      dismissActiveLabel={dismissActiveTabLabel}
      options={tabs.map((tab) => ({
        anchor: tab.anchor,
        icon: tab.icon,
        label: tab.label,
        value: tab.key,
      }))}
    />
  );

  return (
    <div
      className={cn(
        SURFACE_HEADER_CLASS_NAME,
        usesSingleRow && "workspace-surface-header-single-row",
        usesSingleRow && tabsLeading && "workspace-surface-header-with-session-tabs",
        usesSingleRow && COMPACT_WORKSPACE_HEADER_SINGLE_ROW_HEIGHT_CLASS,
      )}
      data-density={density}
      data-layout={usesSingleRow ? "single-row" : "stacked"}
    >
      <div className={cn(
        "flex min-w-0 items-center justify-between px-5 xl:px-6",
        usesSingleRow
          ? "h-full gap-3"
          : cn(WORKSPACE_HEADER_DEFAULT_HEIGHT_CLASS, "gap-3"),
      )}>
        <div className={cn(
          "flex min-w-0 items-center",
          density === "compact" ? "gap-2.5" : "gap-3",
          usesSingleRow ? "workspace-surface-header-single-row-title shrink" : "flex-1",
        )}>
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
            <div className={cn(
              "flex min-w-0 items-center",
              usesSingleRow ? "flex-nowrap gap-x-1.5" : "flex-wrap",
              !usesSingleRow && "gap-x-2 gap-y-1",
            )}>
              <div className={cn(
                "truncate font-black tracking-normal text-(--text-strong)",
                density === "compact" ? "text-[18px]" : "text-[21px]",
              )}>
                {title}
              </div>
              {badge ? (
                <span className="workspace-surface-header-badge shrink-0 rounded-[5px] border border-(--divider-subtle-color) px-1.5 py-0.5 text-[9.5px] font-semibold leading-none text-(--text-soft)">
                  {badge}
                </span>
              ) : null}
              {titleTrailing ? (
                <div className={cn(
                  "min-w-0 shrink text-(--text-default)",
                  usesSingleRow && "workspace-surface-header-single-row-title-trailing max-h-6 overflow-hidden",
                )}>
                  {titleTrailing}
                </div>
              ) : null}
            </div>
            {primarySubtitle ? (
              <div className="mt-1 text-[12px] text-(--text-soft)">
                {primarySubtitle}
              </div>
            ) : null}
          </div>
        </div>

        {usesSingleRow ? (
          <div className="flex min-w-0 flex-1 items-center gap-3">
            {tabsLeading ? (
              <div className="min-w-[180px] flex-1">{tabsLeading}</div>
            ) : compactSubtitle ? (
              <div className="workspace-surface-header-subtitle min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-soft)">
                {compactSubtitle}
              </div>
            ) : null}

            {tabs.length > 0 ? (
              <>
                {tabsLeading ? (
                  <div className="workspace-surface-header-view-tabs h-5 w-px shrink-0 bg-(--divider-subtle-color)" />
                ) : null}
                {renderTabsNav(
                  cn(
                    "workspace-surface-header-view-tabs min-w-0 overflow-x-auto",
                    tabsLeading ? "shrink-0" : "flex-1",
                  ),
                  "视图切换",
                )}
              </>
            ) : null}

            {tabsTrailing ? (
              <div className="shrink-0">{tabsTrailing}</div>
            ) : null}
          </div>
        ) : null}

        {trailing ? (
          <div className={cn(
            "workspace-surface-header-trailing ml-3 flex shrink-0 items-center justify-end",
            usesSingleRow ? "flex-nowrap" : "flex-wrap",
            density === "compact" ? "gap-1.5" : "gap-2",
          )}>
            {trailing}
          </div>
        ) : null}
      </div>

      {hasSecondaryRow ? (
        <div className={cn(
          "flex min-w-0",
          tabsLeading ? "px-3 xl:px-4" : "px-5 xl:px-6",
          "items-end gap-4 pb-0.5",
        )}>
          {tabsLeading ? (
            <div className="min-w-0 flex-1">{tabsLeading}</div>
          ) : tabs.length > 0 ? (
            renderTabsNav(
              cn(
                "soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 overflow-x-auto px-0.5",
                "items-center gap-4",
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
                  "items-center gap-4",
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
