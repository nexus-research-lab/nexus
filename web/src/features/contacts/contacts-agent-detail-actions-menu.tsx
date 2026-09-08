// INPUT: 当前 Agent 身份/名称与聊天、建群、删除确认入口。
// OUTPUT: 精确身份绑定的窄窗动作菜单，身份变化关闭旧菜单，未知值不派发命令。
// POS: Contacts 详情窄窗操作装配；删除确认和业务事务继续由页面拥有。

import { MessageCirclePlus, MessageSquareText, MoreHorizontal, Trash2 } from "lucide-react";
import { useRef } from "react";

import { getAgentDisplayName } from "@/lib/agent-display-name";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";

interface ContactsAgentDetailActionsMenuProps {
  agentId: string;
  agentName: string;
  onCreateTeam: () => void;
  onDelete: () => void;
  onOpenDirectRoom: () => void;
}

export function ContactsAgentDetailActionsMenu({
  agentId,
  agentName,
  onCreateTeam,
  onDelete,
  onOpenDirectRoom,
}: ContactsAgentDetailActionsMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useResettableState(false, agentId);
  const actionLabel = t("contacts.agent_actions", { name: getAgentDisplayName(agentName, t) });
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
        aria-label={actionLabel}
        onClick={() => setIsOpen((current) => !current)}
        size="md"
        tooltip={actionLabel}
        variant="ghost"
      >
        <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
      </UiIconButton>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={actionLabel}
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
          if (value === "delete") onDelete();
        }}
      />
    </>
  );
}
