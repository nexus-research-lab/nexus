// INPUT: 上层派生的一级导航项、当前入口与选择动作。
// OUTPUT: 复用统一 Dock 动作的聊天、联系人和能力导航轨。
// POS: 宽侧栏一级导航纯视图；不读取路由、Store 或业务 API。

import { SidebarRailAction } from "./sidebar-rail-action";
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
    <div className="flex flex-col items-center gap-1.5 px-1 py-2">
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
  return (
    <SidebarRailAction
      active={active}
      badgeCount={item.badgeCount}
      data-tour-anchor={item.anchor}
      icon={item.icon}
      label={item.label}
      layout="primary"
      onClick={() => onSelect(item.key)}
    />
  );
}
