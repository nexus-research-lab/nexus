"use client";

import { MoreHorizontal } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { buildRoomMobileActionItems } from "./room-mobile-actions-model";

type RoomMobileAuxiliaryTab = Exclude<RoomSurfaceTabKey, "chat">;

interface RoomMobileActionsMenuProps {
  canOpenSubagents: boolean;
  onCreateConversation: () => Promise<string | null>;
  onManageMembers?: () => void;
  onOpenAuxiliaryTab: (tab: RoomMobileAuxiliaryTab) => void;
}

export function RoomMobileActionsMenu({
  canOpenSubagents,
  onCreateConversation,
  onManageMembers,
  onOpenAuxiliaryTab,
}: RoomMobileActionsMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const items = buildRoomMobileActionItems({
    canOpenSubagents,
    includeMembers: Boolean(onManageMembers),
    t,
  });

  return (
    <>
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        className="inline-flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
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
        minWidth={190}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === "new_conversation") {
            void onCreateConversation();
            return;
          }
          if (value === "members") {
            onManageMembers?.();
            return;
          }
          onOpenAuxiliaryTab(value as RoomMobileAuxiliaryTab);
        }}
      />
    </>
  );
}
