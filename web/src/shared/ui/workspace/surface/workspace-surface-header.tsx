"use client";

import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiUnderlineTabs } from "@/shared/ui/navigation/tabs";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

import "./workspace-surface-header.css";

const SURFACE_HEADER_CLASS_NAME =
  "workspace-surface-header border-b border-(--divider-subtle-color) bg-transparent";

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
  dismissActiveTabLabel?: string;
  leading?: ReactNode;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
  title?: string;
  titleTrailing?: ReactNode;
  trailing?: ReactNode;
} & WorkspaceSurfaceHeaderMiddle;

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  activeTab,
  badge,
  dismissActiveTabLabel,
  leading,
  onChangeTab,
  onDismissActiveTab,
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
      <div className="flex h-full min-w-0 items-center justify-between gap-3 px-5 xl:px-6">
        <WorkspaceSurfaceIdentity
          badge={badge}
          leading={leading}
          title={title}
          titleTrailing={titleTrailing}
        />

        <WorkspaceSurfaceNavigation
          activeTab={activeTab}
          dismissActiveTabLabel={dismissActiveTabLabel}
          onChangeTab={onChangeTab}
          onDismissActiveTab={onDismissActiveTab}
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
  title,
  titleTrailing,
}: {
  badge?: string;
  leading?: ReactNode;
  title?: string;
  titleTrailing?: ReactNode;
}) {
  const hasTitleContent = Boolean(title) || Boolean(badge) || Boolean(titleTrailing);

  return (
    <div className="workspace-surface-header-title flex min-w-0 shrink items-center gap-2.5">
      {leading ? (
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
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
    <div className="flex min-w-0 flex-1 flex-nowrap items-center gap-x-1.5">
      {title ? (
        <div className="truncate text-[17px] font-semibold leading-5 tracking-normal text-(--text-strong)">
          {title}
        </div>
      ) : null}
      {badge ? (
        <span className="workspace-surface-header-badge shrink-0 rounded-[5px] border border-(--divider-subtle-color) px-1.5 py-0.5 text-[9.5px] font-semibold leading-none text-(--text-soft)">
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
  dismissActiveTabLabel,
  onChangeTab,
  onDismissActiveTab,
  subtitle,
  tabs,
  tabsLeading,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  dismissActiveTabLabel?: string;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  subtitle?: ReactNode;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsLeading?: ReactNode;
  tabsNavAnchor?: string;
}) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3">
      <WorkspaceSurfaceNavigationLead
        subtitle={subtitle}
        tabsLeading={tabsLeading}
      />
      <WorkspaceSurfaceNavigationDivider
        visible={Boolean(tabsLeading) && tabs.length > 0}
      />
      <WorkspaceSurfaceTabs
        activeTab={activeTab}
        dismissActiveTabLabel={dismissActiveTabLabel}
        hasLeading={Boolean(tabsLeading)}
        onChangeTab={onChangeTab}
        onDismissActiveTab={onDismissActiveTab}
        tabs={tabs}
        tabsNavAnchor={tabsNavAnchor}
      />
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
    return <div className="min-w-[180px] flex-1">{tabsLeading}</div>;
  }
  if (!subtitle) return null;

  return (
    <div className="workspace-surface-header-subtitle min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-soft)">
      {subtitle}
    </div>
  );
}

function WorkspaceSurfaceNavigationDivider({ visible }: { visible: boolean }) {
  if (!visible) return null;

  return (
    <div className="workspace-surface-header-view-tabs h-5 w-px shrink-0 bg-(--divider-subtle-color)" />
  );
}

function WorkspaceSurfaceTabs<TTabKey extends string>({
  activeTab,
  dismissActiveTabLabel,
  hasLeading,
  onChangeTab,
  onDismissActiveTab,
  tabs,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  dismissActiveTabLabel?: string;
  hasLeading: boolean;
  onChangeTab?: (tab: TTabKey) => void;
  onDismissActiveTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
}) {
  if (tabs.length === 0) return null;

  return (
    <UiUnderlineTabs
      activeValue={activeTab}
      ariaLabel="视图切换"
      className={cn(
        "workspace-surface-header-view-tabs min-w-0 overflow-x-auto",
        hasLeading ? "shrink-0" : "flex-1",
      )}
      density="compact"
      dismissActiveLabel={dismissActiveTabLabel}
      navAnchor={tabsNavAnchor}
      onChange={onChangeTab}
      onDismissActive={onDismissActiveTab}
      options={tabs.map((tab) => ({
        anchor: tab.anchor,
        icon: tab.icon,
        label: tab.label,
        value: tab.key,
      }))}
    />
  );
}

function WorkspaceSurfaceTrailing({ children }: { children?: ReactNode }) {
  if (!children) return null;

  return (
    <div className="workspace-surface-header-trailing ml-3 flex shrink-0 flex-nowrap items-center justify-end gap-1.5">
      {children}
    </div>
  );
}
