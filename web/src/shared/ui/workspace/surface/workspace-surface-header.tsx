// INPUT: Workspace 身份、标题、导航标签、窄窗策略与业务动作插槽。
// OUTPUT: 统一内容轴、语义排版、身份外形和响应式导航的 Workspace Header。
// POS: Workspace 顶部导航原语；不拥有业务标签、当前选择或动作事务。

"use client";

import { ChevronDown, type LucideIcon } from "lucide-react";
import { useRef, useState, type ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_CONTENT_GUTTER_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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

type WorkspaceSurfaceHeaderLeadingVariant = "identity" | "section";
type WorkspaceSurfaceHeaderNarrowMode = "full" | "hidden" | "toolbar";

type WorkspaceSurfaceHeaderMiddle =
  | { subtitle?: ReactNode; tabsLeading?: never }
  | { subtitle?: never; tabsLeading: ReactNode };

type WorkspaceSurfaceHeaderProps<TTabKey extends string> = {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  leading?: ReactNode;
  leadingClassName?: string;
  leadingVariant?: WorkspaceSurfaceHeaderLeadingVariant;
  onChangeTab?: (tab: TTabKey) => void;
  navigationTrailing?: ReactNode;
  narrowMode?: WorkspaceSurfaceHeaderNarrowMode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
  title?: string;
  titleTrailing?: ReactNode;
  trailing?: ReactNode;
} & WorkspaceSurfaceHeaderMiddle;

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  leading,
  leadingClassName,
  leadingVariant = "section",
  onChangeTab,
  navigationTrailing,
  narrowMode = "full",
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
        narrowMode === "hidden" && "workspace-surface-header-narrow-hidden",
        narrowMode === "toolbar" && "workspace-surface-header-narrow-toolbar",
        WORKSPACE_HEADER_HEIGHT_CLASS,
      )}
      data-desktop-window-drag-region
    >
      <div className={cn(
        WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
        "workspace-surface-header-inner flex h-full min-w-0 items-center justify-between",
      )}>
        <WorkspaceSurfaceIdentity
          leading={leading}
          leadingClassName={leadingClassName}
          leadingVariant={leadingVariant}
          title={title}
          titleTrailing={titleTrailing}
        />

        <WorkspaceSurfaceNavigation
          activeTab={activeTab}
          compactTabsLabel={compactTabsLabel}
          onChangeTab={onChangeTab}
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
  leading,
  leadingClassName,
  leadingVariant,
  title,
  titleTrailing,
}: {
  leading?: ReactNode;
  leadingClassName?: string;
  leadingVariant: WorkspaceSurfaceHeaderLeadingVariant;
  title?: string;
  titleTrailing?: ReactNode;
}) {
  const hasTitleContent = Boolean(title) || Boolean(titleTrailing);
  if (!leading && !hasTitleContent) return null;

  return (
    <div className="workspace-surface-header-title flex min-w-0 shrink items-center gap-2.5">
      {leading ? (
        <div className={cn(
          "workspace-surface-header-leading flex shrink-0 items-center justify-center text-(--icon-default)",
          leadingVariant === "identity"
            ? "workspace-surface-header-identity-avatar h-10 w-10 radius-control-md border border-(--surface-avatar-border) bg-(--surface-avatar-background)"
            : "workspace-surface-header-section-icon h-8 w-8 radius-control-sm bg-(--surface-interactive-hover-background)",
          leadingClassName,
        )}>
          {leading}
        </div>
      ) : null}

      {hasTitleContent ? (
        <WorkspaceSurfaceTitle
          title={title}
          titleTrailing={titleTrailing}
        />
      ) : null}
    </div>
  );
}

function WorkspaceSurfaceTitle({
  title,
  titleTrailing,
}: {
  title?: string;
  titleTrailing?: ReactNode;
}) {
  return (
    <div className="workspace-surface-header-title-content flex min-w-0 flex-1 flex-nowrap items-center gap-x-1.5">
      {title ? (
        <div className={cn(
          "truncate",
          getUiTypographyClassName({ role: "pageTitle", tone: "strong" }),
        )}>
          {title}
        </div>
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
  onChangeTab,
  navigationTrailing,
  subtitle,
  tabs,
  tabsLeading,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  onChangeTab?: (tab: TTabKey) => void;
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
            hasLeading={Boolean(tabsLeading)}
            onChangeTab={onChangeTab}
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
    <div className={cn(
      "workspace-surface-header-subtitle min-w-0 flex-1 truncate",
      getUiTypographyClassName({ role: "metadata", tone: "soft" }),
    )}>
      {subtitle}
    </div>
  );
}

function WorkspaceSurfaceTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  hasLeading,
  onChangeTab,
  tabs,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  hasLeading: boolean;
  onChangeTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
}) {
  const { t } = useI18n();
  if (tabs.length === 0) return null;

  return (
    <>
      <UiTabs
        activeValue={activeTab}
        ariaLabel={t("common.view_switcher")}
        className={cn(
          "workspace-surface-header-view-tabs min-w-0 overflow-visible",
          hasLeading ? "shrink-0" : "flex-1",
        )}
        density="compact"
        navAnchor={tabsNavAnchor}
        onChange={onChangeTab}
        itemClassName="workspace-surface-header-view-tab"
        options={tabs.map((tab) => ({
          anchor: tab.anchor,
          className: `workspace-surface-header-view-tab-item workspace-surface-header-view-tab-item-${tab.key}`,
          icon: tab.icon,
          label: (
            <span className="workspace-surface-header-view-tab-label">
              {tab.label}
            </span>
          ),
          title: tab.label,
          value: tab.key,
        }))}
      />
      <WorkspaceSurfaceCompactTabs
        activeTab={activeTab}
        compactTabsLabel={compactTabsLabel ?? tabs[0].label}
        onChangeTab={onChangeTab}
        tabs={tabs}
        tabsNavAnchor={tabsNavAnchor}
      />
    </>
  );
}

function WorkspaceSurfaceCompactTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  onChangeTab,
  tabs,
  tabsNavAnchor,
}: {
  activeTab?: TTabKey;
  compactTabsLabel: string;
  onChangeTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsNavAnchor?: string;
}) {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const activeOption = tabs.find((tab) => tab.key === activeTab);
  const ActiveIcon = activeOption?.icon;
  const triggerLabel = activeOption?.label ?? compactTabsLabel;

  return (
    <div
      className={cn(
        "workspace-surface-header-compact-tabs h-8 min-w-0 items-center overflow-hidden radius-control-sm border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_55%,transparent)]",
        activeOption && "border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)]",
      )}
      data-tour-anchor={tabsNavAnchor}
    >
      <UiButton
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={compactTabsLabel}
        className={cn(
          "h-full min-h-0 min-w-0 radius-control-sm px-2 py-0",
          getUiTypographyClassName({ role: "caption", tone: "default", weight: "semibold" }),
        )}
        onClick={() => setIsOpen((current) => !current)}
        size="xs"
        title={triggerLabel}
        variant="ghost"
      >
        {ActiveIcon ? <ActiveIcon className="h-3.5 w-3.5 shrink-0" /> : null}
        <span className="workspace-surface-header-compact-tabs-label min-w-0 truncate">
          {triggerLabel}
        </span>
        <ChevronDown className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      </UiButton>
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
