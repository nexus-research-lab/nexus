import { UiCounterBadge } from "@/shared/ui/display/badge";

import {
  getSidebarPrimaryTabsClassName,
  resolveSidebarPrimaryTabPresentation,
} from "./sidebar-primary-tabs-model";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
} from "./sidebar-wide-panel-types";

interface SidebarPrimaryTabsProps {
  activeTab: SidebarPrimaryTab | null;
  items: SidebarPrimaryTabItem[];
  onSelect: (tab: SidebarPrimaryTab) => void;
}

export function SidebarPrimaryTabs({
  activeTab,
  items,
  onSelect,
}: SidebarPrimaryTabsProps) {
  return (
    <div className={getSidebarPrimaryTabsClassName()}>
      {items.map((item) => (
        <PrimaryTabButton
          active={activeTab === item.key}
          item={item}
          key={item.key}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}

function PrimaryTabButton({
  active,
  item,
  onSelect,
}: {
  active: boolean;
  item: SidebarPrimaryTabItem;
  onSelect: (tab: SidebarPrimaryTab) => void;
}) {
  const Icon = item.icon;
  const presentation = resolveSidebarPrimaryTabPresentation({
    active,
  });
  return (
    <button
      aria-current={presentation.ariaCurrent}
      aria-pressed={active}
      className={presentation.buttonClassName}
      data-tour-anchor={item.anchor}
      onClick={() => onSelect(item.key)}
      type="button"
    >
      <span className={presentation.iconFrameClassName}>
        <Icon className={presentation.iconClassName} />
        <UiCounterBadge
          className={presentation.badgeClassName}
          count={item.badgeCount}
        />
      </span>
      {presentation.showLabel ? (
        <span className={presentation.labelClassName}>{item.label}</span>
      ) : null}
    </button>
  );
}
