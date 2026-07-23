import { MessageSquareText, MoreHorizontal, Users } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";

interface ContactsAgentDetailActionsMenuProps {
  onCreateTeam: () => void;
  onOpenDirectRoom: () => void;
}

export function ContactsAgentDetailActionsMenu({
  onCreateTeam,
  onOpenDirectRoom,
}: ContactsAgentDetailActionsMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const items: UiActionMenuItem[] = [
    {
      icon: <MessageSquareText className="h-4 w-4 text-(--icon-muted)" />,
      label: t("contacts.chat"),
      tone: "primary",
      value: "chat",
    },
    {
      icon: <Users className="h-4 w-4 text-(--icon-muted)" />,
      label: t("contacts.create_team"),
      value: "team",
    },
  ];

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
        minWidth={176}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === "chat") {
            onOpenDirectRoom();
            return;
          }
          onCreateTeam();
        }}
      />
    </>
  );
}
