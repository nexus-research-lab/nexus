"use client";

import { Bot, MessageSquareText, Users } from "lucide-react";

import { getIconAvatarSrc } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceIconFrame,
  WorkspaceCatalogTextAction,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/workspace-catalog-card";

interface ContactsAgentCardProps {
  /** Agent 名称 */
  name: string;
  avatar?: string | null;
  /** Agent 描述（system_prompt 摘要） */
  description: string;
  /** 点击卡片本身 → 打开 AgentOptions 对话框（edit 模式） */
  on_open_profile: () => void;
  /** 💬 Chat 按钮 → ensureDirectRoom 发起 DM */
  on_open_room: () => void;
  /** 👥 Create Team 按钮 → 用该 Agent 创建 Room */
  on_create_team: () => void;
}

/** Agent 卡片 — 居中布局，底部动作收为轻量文本按钮，避免主区继续堆胶囊层。 */
export function ContactsAgentCard({
  name,
  avatar,
  description,
  on_open_profile,
  on_open_room,
  on_create_team,
}: ContactsAgentCardProps) {
  const { t } = useI18n();
  return (
    <WorkspaceCatalogCard
      align="center"
      class_name="h-full"
      interactive
      onClick={on_open_profile}
      size="comfort"
    >
      <WorkspaceIconFrame class_name="mx-auto h-16 w-16 overflow-hidden" shape="round" size="lg">
        {getIconAvatarSrc(avatar) ? (
          <img
            alt={name}
            className="h-full w-full object-cover"
            src={getIconAvatarSrc(avatar) ?? undefined}
          />
        ) : (
          <Bot className="h-7 w-7 text-[color:var(--icon-strong)]" />
        )}
      </WorkspaceIconFrame>

      <WorkspaceCatalogBody class_name="mt-4 w-full" grow={false}>
        <WorkspaceCatalogTitle size="lg" truncate>
        {name}
        </WorkspaceCatalogTitle>
        <WorkspaceCatalogDescription class_name="mt-2" min_height>
          {description}
        </WorkspaceCatalogDescription>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter class_name="mt-5 w-full gap-3" justify="center" onClick={(e) => e.stopPropagation()}>
        <WorkspaceCatalogTextAction onClick={on_open_room} tone="primary">
          <MessageSquareText className="h-3.5 w-3.5" />
          {t("contacts.chat")}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={on_create_team}>
          <Users className="h-3.5 w-3.5" />
          {t("contacts.create_team")}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
