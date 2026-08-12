/**
 * INPUT: 移动端子智能体/成员能力与 i18n 投影。
 * OUTPUT: 顺序稳定、工作图常驻的移动端更多操作项。
 * POS: Room 移动端操作菜单的纯视图模型。
 */
import {
  Bot,
  FolderTree,
  Info,
  Plus,
  UsersRound,
  Workflow,
} from "lucide-react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { UiActionMenuItem } from "@/shared/ui/menu/action-menu";

export function buildRoomMobileActionItems({
  canOpenSubagents,
  includeMembers,
  t,
}: {
  canOpenSubagents: boolean;
  includeMembers: boolean;
  t: I18nContextValue["t"];
}): UiActionMenuItem[] {
  return [
    {
      icon: <Plus className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.new_conversation"),
      tone: "primary",
      value: "new_conversation",
    },
    ...(includeMembers ? [{
      icon: <UsersRound className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.members"),
      value: "members",
    }] : []),
    {
      icon: <Workflow className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.workgraph"),
      value: "workgraph",
    },
    {
      disabled: !canOpenSubagents,
      icon: <Bot className="h-4 w-4 text-(--icon-muted)" />,
      label: t("subagents.label"),
      value: "subagents",
    },
    {
      icon: <FolderTree className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.workspace"),
      value: "workspace",
    },
    {
      icon: <Info className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.about"),
      value: "about",
    },
  ];
}
