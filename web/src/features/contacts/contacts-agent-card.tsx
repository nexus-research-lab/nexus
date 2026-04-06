"use client";

import { Bot, MessageSquareText, Users } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceCatalogCard,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";

interface ContactsAgentCardProps {
  /** Agent 名称 */
  name: string;
  /** Agent 描述（system_prompt 摘要） */
  description: string;
  /** 点击卡片本身 → 打开 AgentOptions 对话框（edit 模式） */
  on_open_profile: () => void;
  /** 💬 Chat 按钮 → ensureDirectRoom 发起 DM */
  on_open_room: () => void;
  /** 👥 Create Team 按钮 → 用该 Agent 创建 Room */
  on_create_team: () => void;
}

/** Agent 卡片 — 居中布局，底部 Chat / Create Team 双按钮 */
export function ContactsAgentCard({
  name,
  description,
  on_open_profile,
  on_open_room,
  on_create_team,
}: ContactsAgentCardProps) {
  const { t } = useI18n();
  return (
    <WorkspaceCatalogCard
      class_name="cursor-pointer rounded-[26px] px-6 py-6 text-center"
      onClick={on_open_profile}
    >
      {/* 居中头像 */}
      <WorkspaceIconFrame class_name="mx-auto h-16 w-16" shape="round" size="lg">
        <Bot className="h-7 w-7 text-slate-900/88" />
      </WorkspaceIconFrame>

      {/* 名称 */}
      <p className="mt-4 truncate text-[18px] font-bold tracking-[-0.03em] text-slate-950/96">
        {name}
      </p>

      {/* 描述：1-2 行截断 */}
      <p className="mt-2 line-clamp-2 min-h-[40px] text-[13px] leading-5 text-slate-700/88">
        {description}
      </p>

      {/* 底部操作按钮 */}
      <div className="mt-5 flex items-center justify-center gap-2.5" onClick={(e) => e.stopPropagation()}>
        <WorkspacePillButton onClick={on_open_room} size="sm" variant="primary">
          <MessageSquareText className="h-3.5 w-3.5" />
          {t("contacts.chat")}
        </WorkspacePillButton>
        <WorkspacePillButton onClick={on_create_team} size="sm" variant="outlined">
          <Users className="h-3.5 w-3.5" />
          {t("contacts.create_team")}
        </WorkspacePillButton>
      </div>
    </WorkspaceCatalogCard>
  );
}
