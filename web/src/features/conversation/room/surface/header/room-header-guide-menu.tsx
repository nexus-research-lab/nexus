import { Compass, MoreHorizontal, UsersRound, X } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";

import type {
  RoomHeaderTab,
  RoomSurfaceTabKey,
} from "./room-header-tabs";

interface RoomHeaderGuideMenuProps {
  activeTab?: RoomSurfaceTabKey;
  collapsedTabs?: RoomHeaderTab[];
  onChangeTab?: (tab: RoomSurfaceTabKey) => void;
  onCloseActiveTab?: () => void;
  onManageMembers?: () => void;
  onReplayTour?: () => void;
}

export function RoomHeaderGuideMenu({
  activeTab,
  collapsedTabs = [],
  onChangeTab,
  onCloseActiveTab,
  onManageMembers,
  onReplayTour,
}: RoomHeaderGuideMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const collapsedTabItems: UiActionMenuItem[] = collapsedTabs.map((tab) => {
    const Icon = tab.icon;
    const isActive = tab.key === activeTab;
    return {
      active: isActive,
      icon: <Icon className="h-4 w-4 text-(--icon-muted)" />,
      label: tab.label,
      trailing: isActive ? <X className="h-3.5 w-3.5 text-(--icon-muted)" /> : undefined,
      value: `tab:${tab.key}`,
    };
  });
  const items: UiActionMenuItem[] = [
    ...collapsedTabItems,
    onManageMembers ? {
      icon: <UsersRound className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.members"),
      value: "members",
    } : null,
    onReplayTour ? {
      icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
      label: t("common.view_guide"),
      value: "guide",
    } : null,
  ].filter((item): item is UiActionMenuItem => Boolean(item));

  if (items.length === 0) {
    return null;
  }

  return (
    <>
      <button
        ref={buttonRef}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        className="inline-flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={() => setIsOpen((current) => !current)}
        title={t("common.more_actions")}
        type="button"
      >
        <MoreHorizontal className="h-4 w-4" />
      </button>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("common.more_actions")}
        isOpen={isOpen}
        items={items}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          const collapsedTab = collapsedTabs.find(
            (tab) => value === `tab:${tab.key}`,
          );
          if (collapsedTab) {
            if (collapsedTab.key === activeTab) {
              onCloseActiveTab?.();
              return;
            }
            onChangeTab?.(collapsedTab.key);
            return;
          }
          if (value === "members") {
            onManageMembers?.();
            return;
          }
          onReplayTour?.();
        }}
      />
    </>
  );
}
