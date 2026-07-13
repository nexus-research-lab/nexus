import { UiCounterBadge } from "@/shared/ui/display/badge";

import {
  getSidebarPrimaryTabsClassName,
  resolveSidebarPrimaryTabPresentation,
  type SidebarPrimaryTabsVariant,
} from "./sidebar-primary-tabs-model";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
} from "./sidebar-wide-panel-types";

interface SidebarPrimaryTabsProps {
  activeTab: SidebarPrimaryTab;
  items: SidebarPrimaryTabItem[];
  onSelect: (tab: SidebarPrimaryTab) => void;
  variant: SidebarPrimaryTabsVariant;
}

export function SidebarPrimaryTabs({
  activeTab,
  items,
  onSelect,
  variant,
}: SidebarPrimaryTabsProps) {
  return (
    <div className={getSidebarPrimaryTabsClassName(variant)}>
      {items.map((item) => (
        <PrimaryTabButton
          active={activeTab === item.key}
          item={item}
          key={item.key}
          onSelect={onSelect}
          variant={variant}
        />
      ))}
    </div>
  );
}

function PrimaryTabButton({
  active,
  item,
  onSelect,
  variant,
}: {
  active: boolean;
  item: SidebarPrimaryTabItem;
  onSelect: (tab: SidebarPrimaryTab) => void;
  variant: SidebarPrimaryTabsProps["variant"];
}) {
  const Icon = item.icon;
  const presentation = resolveSidebarPrimaryTabPresentation({
    active,
    label: item.label,
    variant,
  });
  return (
    <button
      aria-current={presentation.ariaCurrent}
      aria-label={presentation.ariaLabel}
      aria-pressed={active}
      className={presentation.buttonClassName}
      data-tour-anchor={item.anchor}
      onClick={() => onSelect(item.key)}
      title={item.label}
      type="button"
    >
      <span className={presentation.iconFrameClassName}>
        <Icon className={presentation.iconClassName} />
        <UiCounterBadge
          className={presentation.badgeClassName}
          count={item.badgeCount}
        />
      </span>
      {presentation.showLabel ? <span>{item.label}</span> : null}
    </button>
  );
}
