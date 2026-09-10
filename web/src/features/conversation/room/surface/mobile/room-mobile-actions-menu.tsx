// INPUT: Room 窄窗作用域、可用动作、成员加载态与业务命令。
// OUTPUT: 切换作用域即消费打开态的语义动作菜单；只分派明确动作。
// POS: Room 专注模式尾部动作装配；不拥有菜单项规则或业务事务。

"use client";

import { MoreHorizontal } from "lucide-react";
import { useRef } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

import { buildRoomMobileActionItems } from "./room-mobile-actions-model";

type RoomMobileAuxiliaryTab = Exclude<RoomSurfaceTabKey, "chat">;

interface RoomMobileActionsMenuProps {
  canOpenSubagents: boolean;
  isMembersLoading: boolean;
  scopeKey: string;
  onCreateConversation: () => Promise<string | null>;
  onManageMembers?: () => void;
  onOpenAuxiliaryTab: (tab: RoomMobileAuxiliaryTab) => void;
}

export function RoomMobileActionsMenu({
  canOpenSubagents,
  isMembersLoading,
  scopeKey,
  onCreateConversation,
  onManageMembers,
  onOpenAuxiliaryTab,
}: RoomMobileActionsMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useResettableState(false, scopeKey);
  const items = buildRoomMobileActionItems({
    canOpenSubagents,
    includeMembers: Boolean(onManageMembers),
    isMembersLoading,
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
        <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
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
            if (!isMembersLoading) onManageMembers?.();
            return;
          }
          if (value === "subagents" && canOpenSubagents) onOpenAuxiliaryTab(value);
          if (value === "about" || value === "workgraph" || value === "workspace") {
            onOpenAuxiliaryTab(value);
          }
        }}
      />
    </>
  );
}
