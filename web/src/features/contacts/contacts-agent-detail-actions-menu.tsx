import { MessageCirclePlus, MessageSquareText, MoreHorizontal, Trash2 } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";

interface ContactsAgentDetailActionsMenuProps {
  onCreateTeam: () => void;
  onDelete: () => void;
  onOpenDirectRoom: () => void;
}

export function ContactsAgentDetailActionsMenu({
  onCreateTeam,
  onDelete,
  onOpenDirectRoom,
}: ContactsAgentDetailActionsMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const items: UiActionMenuItem[] = [
    {
      icon: <MessageSquareText className="h-4 w-4 text-(--icon-muted)" />,
      label: t("contacts.chat"),
      value: "chat",
    },
    {
      icon: <MessageCirclePlus className="h-4 w-4 text-(--icon-muted)" />,
      label: t("contacts.create_team"),
      value: "team",
    },
  ];
  const footerItems: UiActionMenuItem[] = [
    {
      icon: <Trash2 className="h-4 w-4" />,
      label: t("agent_options.delete_agent"),
      tone: "danger",
      value: "delete",
    },
  ];

  return (
    <>
      <UiIconButton
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        onClick={() => setIsOpen((current) => !current)}
        size="md"
        title={t("common.more_actions")}
        type="button"
        variant="ghost"
      >
        <MoreHorizontal className="h-4 w-4" />
      </UiIconButton>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("common.more_actions")}
        footerItems={footerItems}
        isOpen={isOpen}
        items={items}
        minWidth={176}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === "chat") {
            onOpenDirectRoom();
            return;
          }
          if (value === "team") {
            onCreateTeam();
            return;
          }
          onDelete();
        }}
      />
    </>
  );
}
