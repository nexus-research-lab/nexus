// INPUT: Workspace 身份/返回动作、标题、会话或视图导航与业务动作插槽。
// OUTPUT: 统一内容轴与排版的 Header；公共 Select 持有窄窗视图选择，Avatar 持有身份外形。
// POS: Workspace 顶部导航原语；不拥有业务标签、当前选择或动作事务。

"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { type LucideIcon } from "lucide-react";
import { useLayoutEffect, useRef, useState, type ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_CONTENT_GUTTER_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
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

type WorkspaceSurfaceHeaderLeadingVariant = "identity" | "action";
type WorkspaceSurfaceHeaderProps<TTabKey extends string> = {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  leading?: ReactNode;
  leadingVariant?: WorkspaceSurfaceHeaderLeadingVariant;
  onChangeTab?: (tab: TTabKey) => void;
  navigationTrailing?: ReactNode;
  tabs?: WorkspaceSurfaceHeaderTab<TTabKey>[];
  title?: string;
  tabsLeading?: ReactNode;
  trailing?: ReactNode;
};

export function WorkspaceSurfaceHeader<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  leading,
  leadingVariant = "action",
  onChangeTab,
  navigationTrailing,
  tabs = [],
  tabsLeading,
  title,
  trailing,
}: WorkspaceSurfaceHeaderProps<TTabKey>) {
  return (
    <div
      className={cn(
        SURFACE_HEADER_CLASS_NAME,
        tabsLeading && "workspace-surface-header-with-session-tabs",
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
          leadingVariant={leadingVariant}
          title={title}
        />

        <WorkspaceSurfaceNavigation
          activeTab={activeTab}
          compactTabsLabel={compactTabsLabel}
          onChangeTab={onChangeTab}
          navigationTrailing={navigationTrailing}
          tabs={tabs}
          tabsLeading={tabsLeading}
        />

        <WorkspaceSurfaceTrailing>{trailing}</WorkspaceSurfaceTrailing>
      </div>
    </div>
  );
}

function WorkspaceSurfaceIdentity({
  leading,
  leadingVariant,
  title,
}: {
  leading?: ReactNode;
  leadingVariant: WorkspaceSurfaceHeaderLeadingVariant;
  title?: string;
}) {
  if (!leading && !title) return null;
  return (
    <div className="workspace-surface-header-title flex min-w-0 shrink items-center gap-2.5">
      {leading ? (
        <div className={cn(
          "workspace-surface-header-leading flex shrink-0 items-center justify-center",
          leadingVariant === "identity" && "workspace-surface-header-identity-avatar h-10 w-10",
        )}>
          {leading}
        </div>
      ) : null}
      {title ? (
        <UiTooltip label={title}><div className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "pageTitle", tone: "strong" }))} >
          {title}
        </div></UiTooltip>
      ) : null}
    </div>
  );
}

function WorkspaceSurfaceNavigation<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  onChangeTab,
  navigationTrailing,
  tabs,
  tabsLeading,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  onChangeTab?: (tab: TTabKey) => void;
  navigationTrailing?: ReactNode;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
  tabsLeading?: ReactNode;
}) {
  const hasNavigationTools = tabs.length > 0 || Boolean(navigationTrailing);

  return (
    <div className="workspace-surface-header-navigation flex min-w-0 flex-1 items-center">
      {tabsLeading ? (
        <div className="workspace-surface-header-session-tabs min-w-0 flex-1">{tabsLeading}</div>
      ) : null}
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

function WorkspaceSurfaceTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  hasLeading,
  onChangeTab,
  tabs,
}: {
  activeTab?: TTabKey;
  compactTabsLabel?: string;
  hasLeading: boolean;
  onChangeTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
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
        onChange={onChangeTab}
        itemClassName="workspace-surface-header-view-tab"
        options={tabs.map((tab) => ({
          anchor: tab.anchor,
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
      {!hasLeading ? (
        <WorkspaceSurfaceCompactTabs
          activeTab={activeTab}
          compactTabsLabel={compactTabsLabel ?? t("common.view_switcher")}
          onChangeTab={onChangeTab}
          tabs={tabs}
        />
      ) : null}
    </>
  );
}

function WorkspaceSurfaceCompactTabs<TTabKey extends string>({
  activeTab,
  compactTabsLabel,
  onChangeTab,
  tabs,
}: {
  activeTab?: TTabKey;
  compactTabsLabel: string;
  onChangeTab?: (tab: TTabKey) => void;
  tabs: WorkspaceSurfaceHeaderTab<TTabKey>[];
}) {
  const { t } = useI18n();
  const hostRef = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  const activeOption = tabs.find((tab) => tab.key === activeTab);
  const ActiveIcon = activeOption?.icon;
  const resetKey = JSON.stringify([activeTab ?? null, tabs.map((tab) => tab.key).sort()]);

  useLayoutEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    // CSS 拥有 container breakpoint；只观察其实际可见性，避免复制阈值或保留隐藏菜单。
    const updateVisibility = () => setIsVisible(getComputedStyle(host).display !== "none");
    updateVisibility();
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateVisibility);
    observer?.observe(host);
    window.addEventListener("resize", updateVisibility);
    return () => {
      observer?.disconnect();
      window.removeEventListener("resize", updateVisibility);
    };
  }, []);

  return (
    <div ref={hostRef} className="workspace-surface-header-compact-tabs min-w-0">
      <UiSelectMenu
        ariaLabel={activeOption
          ? t("common.view_switcher_current", { label: compactTabsLabel, view: activeOption.label })
          : compactTabsLabel}
        disabled={!isVisible || !onChangeTab}
        leading={ActiveIcon ? <ActiveIcon aria-hidden="true" className="h-3.5 w-3.5" /> : undefined}
        menuMinWidth={176}
        onChange={(value) => {
          const tab = tabs.find((item) => item.key === value);
          if (tab) onChangeTab?.(tab.key);
        }}
        options={tabs.map((tab) => ({ label: tab.label, value: tab.key }))}
        placeholder={compactTabsLabel}
        resetKey={resetKey}
        size="sm"
        value={activeTab ?? ""}
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
