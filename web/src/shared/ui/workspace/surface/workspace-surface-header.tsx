"use client";

import { ChevronDown, X, type LucideIcon } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { UiUnderlineTabs } from "@/shared/ui/navigation/tabs";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

import "./workspace-surface-header.css";

const SURFACE_HEADER_CLASS_NAME =
  "workspace-surface-header shell-region-header";

interface WorkspaceSurfaceHeaderTab<TTabKey extends string> {
  anchor?: string;
  icon?: LucideIcon;
  key: TTabKey;
  label: string;
}

type WorkspaceSurfaceHeaderMiddle =
  | { subtitle?: ReactNode; tabsLeading?: never }
  | { subtitle?: never; tabsLeading: ReactNode };

type WorkspaceSurfaceHeaderProps<TTabKey extends string> = {
  activeTab?: TTabKey;
  badge?: string;
  compactTabsLabel?: string;
  dismissActiveTabLabel?: string;
  leading?: ReactNode;
  leadingClassName?: string;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  navigationTrailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
  title?: string;
  titleTrailing?: ReactNode;
  trailing?: ReactNode;
} & WorkspaceSurfaceHeaderMiddle;

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  activeTab,
  badge,
  compactTabsLabel,
  dismissActiveTabLabel,
  leading,
  leadingClassName,
  onChangeTab,
  onDismissActiveTab,
  navigationTrailing,
  subtitle,
  tabs = [],
  tabsLeading,
  tabsNavAnchor,
  title,
  titleTrailing,
  trailing,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  return (
    <div
      className={cn(
        SURFACE_HEADER_CLASS_NAME,
        tabsLeading && "workspace-surface-header-with-session-tabs",
        WORKSPACE_HEADER_HEIGHT_CLASS,
      )}
    >
      <div className="workspace-surface-header-inner flex h-full min-w-0 items-center justify-between px-5 xl:px-6">
        <WorkspaceSurfaceIdentity
          badge={badge}
          leading={leading}
          leadingClassName={leadingClassName}
          title={title}
          titleTrailing={titleTrailing}
        />

        <WorkspaceSurfaceNavigation
          activeTab={activeTab}
          compactTabsLabel={compactTabsLabel}
          dismissActiveTabLabel={dismissActiveTabLabel}
          onChangeTab={onChangeTab}
          onDismissActiveTab={onDismissActiveTab}
          navigationTrailing={navigationTrailing}
          subtitle={subtitle}
          tabs={tabs}
          tabsLeading={tabsLeading}
          tabsNavAnchor={tabsNavAnchor}
        />

        <WorkspaceSurfaceTrailing>{trailing}</WorkspaceSurfaceTrailing>
      </div>
    </div>
  );
}

function WorkspaceSurfaceIdentity({
  badge,
  leading,
  leadingClassName,
  title,
  titleTrailing,
}: {
  badge?: string;
  leading?: ReactNode;
  leadingClassName?: string;
  title?: string;
  titleTrailing?: ReactNode;
}) {
  const hasTitleContent = Boolean(title) || Boolean(badge) || Boolean(titleTrailing);

  return (
    <div className="workspace-surface-header-title flex min-w-0 shrink items-center gap-2.5">
      {leading ? (
        <div className={cn(
          "workspace-surface-header-identity-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default)",
          leadingClassName,
        )}>
          {leading}
        </div>
      ) : null}

      {hasTitleContent ? (
        <WorkspaceSurfaceTitle
          badge={badge}
          title={title}
          titleTrailing={titleTrailing}
        />
      ) : null}
    </div>
  );
}

function WorkspaceSurfaceTitle({
  badge,
  title,
  titleTrailing,
}: {
  badge?: string;
  title?: string;
  titleTrailing?: ReactNode;
}) {
  return (
    <div className="workspace-surface-header-title-content flex min-w-0 flex-1 flex-nowrap items-center gap-x-1.5">
      {title ? (
        <div className="truncate text-[17px] font-semibold leading-5 tracking-normal text-(--text-strong)">
          {title}
        </div>
      ) : null}
      {badge ? (
        <span className="workspace-surface-header-badge shrink-0 radius-control-xs border border-(--divider-subtle-color) px-1.5 py-0.5 text-[9.5px] font-semibold leading-none text-(--text-soft)">
          {badge}
        </span>
      ) : null}
      {titleTrailing ? (
        <div className="workspace-surface-header-title-trailing min-w-0 max-h-6 shrink overflow-hidden text-(--text-default)">
          {titleTrailing}
        </div>
      ) : null}
    </div>
  );
}

function WorkspaceSurfaceNavigation<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  dismissActiveTabLabel,
  onChangeTab,
  onDismissActiveTab,
  navigationTrailing,
  subtitle,
  tabs,
  tabsLeading,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  dismissActiveTabLabel?: string;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  navigationTrailing?: ReactNode;
  subtitle?: ReactNode;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsLeading?: ReactNode;
  tabsNavAnchor?: string;
}) {
  const hasNavigationTools = tabs.length > 0 || Boolean(navigationTrailing);

  return (
    <div className="workspace-surface-header-navigation flex min-w-0 flex-1 items-center">
      <WorkspaceSurfaceNavigationLead
        subtitle={subtitle}
        tabsLeading={tabsLeading}
      />
      {hasNavigationTools ? (
        <div
          className={cn(
            "workspace-surface-header-tool-cluster flex shrink-0 items-center",
            !tabsLeading && "workspace-surface-header-tool-cluster-page-tabs",
          )}
        >
          <WorkspaceSurfaceTabs
            activeTab={activeTab}
            compactTabsLabel={compactTabsLabel}
            dismissActiveTabLabel={dismissActiveTabLabel}
            hasLeading={Boolean(tabsLeading)}
            onChangeTab={onChangeTab}
            onDismissActiveTab={onDismissActiveTab}
            tabs={tabs}
            tabsNavAnchor={tabsNavAnchor}
          />
          {navigationTrailing ? (
            <div className="workspace-surface-header-navigation-actions flex shrink-0 items-center">
              {navigationTrailing}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function WorkspaceSurfaceNavigationLead({
  subtitle,
  tabsLeading,
}: {
  subtitle?: ReactNode;
  tabsLeading?: ReactNode;
}) {
  if (tabsLeading) {
    return <div className="workspace-surface-header-session-tabs min-w-0 flex-1">{tabsLeading}</div>;
  }
  if (!subtitle) return null;

  return (
    <div className="workspace-surface-header-subtitle min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-soft)">
      {subtitle}
    </div>
  );
}

function WorkspaceSurfaceTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  dismissActiveTabLabel,
  hasLeading,
  onChangeTab,
  onDismissActiveTab,
  tabs,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  dismissActiveTabLabel?: string;
  hasLeading: boolean;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
}) {
  if (tabs.length === 0) return null;

  return (
    <>
      <UiUnderlineTabs
        activeValue={activeTab}
        ariaLabel="视图切换"
        className={cn(
          "workspace-surface-header-view-tabs min-w-0 overflow-visible",
          hasLeading ? "shrink-0" : "flex-1",
        )}
        density="compact"
        dismissActiveLabel={dismissActiveTabLabel}
        navAnchor={tabsNavAnchor}
        onChange={onChangeTab}
        onDismissActive={onDismissActiveTab}
        itemClassName="workspace-surface-header-view-tab"
        options={tabs.map((tab) => ({
          anchor: tab.anchor,
          className: `workspace-surface-header-view-tab-item workspace-surface-header-view-tab-item-${tab.key}`,
          icon: tab.icon,
          label: tab.label,
          title: tab.label,
          value: tab.key,
        }))}
      />
      <WorkspaceSurfaceCompactTabs
        activeTab={activeTab}
        compactTabsLabel={compactTabsLabel ?? tabs[0].label}
        dismissActiveTabLabel={dismissActiveTabLabel}
        onChangeTab={onChangeTab}
        onDismissActiveTab={onDismissActiveTab}
        tabs={tabs}
        tabsNavAnchor={tabsNavAnchor}
      />
    </>
  );
}

function WorkspaceSurfaceCompactTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  dismissActiveTabLabel,
  onChangeTab,
  onDismissActiveTab,
  tabs,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel: string;
  dismissActiveTabLabel?: string;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
}) {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const activeOption = tabs.find((tab) => tab.key === activeTab);
  const ActiveIcon = activeOption?.icon;
  const triggerLabel = activeOption?.label ?? compactTabsLabel;
  const canDismissActive = Boolean(activeOption && activeTab && onDismissActiveTab);

  return (
    <div
      className={cn(
        "workspace-surface-header-compact-tabs h-8 min-w-0 items-center overflow-hidden radius-control-sm border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_55%,transparent)]",
        activeOption && "border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)]",
      )}
      data-tour-anchor={tabsNavAnchor}
    >
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={compactTabsLabel}
        className="flex h-full min-w-0 items-center gap-1.5 px-2 text-[10.5px] font-semibold text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={() => setIsOpen((current) => !current)}
        title={triggerLabel}
        type="button"
      >
        {ActiveIcon ? <ActiveIcon className="h-3.5 w-3.5 shrink-0" /> : null}
        <span className="workspace-surface-header-compact-tabs-label min-w-0 truncate">
          {triggerLabel}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      </button>
      {canDismissActive ? (
        <button
          aria-label={dismissActiveTabLabel}
          className="flex h-full w-7 shrink-0 items-center justify-center border-l border-(--divider-subtle-color) text-(--icon-muted) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          onClick={() => onDismissActiveTab?.(activeTab as TTabKey)}
          title={dismissActiveTabLabel}
          type="button"
        >
          <X className="h-3 w-3" />
        </button>
      ) : null}
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={compactTabsLabel}
        isOpen={isOpen}
        items={tabs.map((tab) => {
          const Icon = tab.icon;
          return {
            active: tab.key === activeTab,
            icon: Icon ? <Icon className="h-4 w-4 text-(--icon-muted)" /> : undefined,
            label: tab.label,
            value: tab.key,
          };
        })}
        minWidth={176}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => onChangeTab?.(value as TTabKey)}
      />
    </div>
  );
}

function WorkspaceSurfaceTrailing({ children }: { children?: ReactNode }) {
  if (!children) return null;

  return (
    <div className="workspace-surface-header-trailing flex shrink-0 flex-nowrap items-center justify-end">
      {children}
    </div>
  );
}
