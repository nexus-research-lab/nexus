"use client";

import { Bot, MessageSquareText, Users } from "lucide-react";

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
  return (
    <article
      className="workspace-card cursor-pointer rounded-[26px] border border-white/24 px-6 py-6 text-center transition-all hover:border-white/30 hover:bg-white/34"
      onClick={on_open_profile}
    >
      {/* 居中头像 */}
      <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-white/44 bg-white/64">
        <Bot className="h-7 w-7 text-slate-900/88" />
      </div>

      {/* 名称 */}
      <p className="mt-4 truncate text-[18px] font-bold tracking-[-0.03em] text-slate-950/92">
        {name}
      </p>

      {/* 描述：1-2 行截断 */}
      <p className="mt-2 line-clamp-2 min-h-[40px] text-[13px] leading-5 text-slate-700/68">
        {description}
      </p>

      {/* 底部操作按钮 */}
      <div className="mt-5 flex items-center justify-center gap-3" onClick={(e) => e.stopPropagation()}>
        <WorkspacePillButton onClick={on_open_room} size="sm" variant="success">
          <MessageSquareText className="h-3.5 w-3.5" />
          Chat
        </WorkspacePillButton>
        <WorkspacePillButton onClick={on_create_team} size="sm">
          <Users className="h-3.5 w-3.5" />
          Create Team
        </WorkspacePillButton>
      </div>
    </article>
  );
}
