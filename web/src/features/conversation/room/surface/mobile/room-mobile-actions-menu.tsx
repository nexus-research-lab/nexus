// INPUT: Room 窄窗可用动作、业务命令与辅助页打开入口。
// OUTPUT: 共享圆形图标按钮触发的语义动作菜单。
// POS: Room 专注模式尾部动作装配；不拥有菜单项规则或业务事务。

"use client";

import { MoreHorizontal } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
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
      <UiIconButton
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        onClick={() => setIsOpen((current) => !current)}
        shape="round"
        size="lg"
        variant="ghost"
      >
        <MoreHorizontal className="h-4 w-4" />
      </UiIconButton>
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
