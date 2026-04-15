"use client";

import { Bot, MessageSquareText, Users } from "lucide-react";

import { get_icon_avatar_src } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { Agent } from "@/types/agent";
import { format_provider_label } from "@/types/provider";
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
  agent: Agent;
  /** 点击卡片本身 → 打开 AgentOptions 对话框（edit 模式） */
  on_open_profile: () => void;
  /** 💬 Chat 按钮 → ensureDirectRoom 发起 DM */
  on_open_room: () => void;
  /** 👥 Create Team 按钮 → 用该 Agent 创建 Room */
  on_create_team: () => void;
}

/** Agent 卡片 — 居中布局，底部动作收为轻量文本按钮，避免主区继续堆胶囊层。 */
export function ContactsAgentCard({
  agent,
  on_open_profile,
  on_open_room,
  on_create_team,
}: ContactsAgentCardProps) {
  const { t } = useI18n();

  // 提取配置信息
  const permissionMode = agent.options.permission_mode || "default";
  const provider = format_provider_label(agent.options.provider);
  const allowedToolsCount = agent.options.allowed_tools?.length || 0;
  const skillsCount = agent.skills_count || 0;

  return (
    <WorkspaceCatalogCard
      align="center"
      class_name="relative h-full overflow-hidden"
      interactive
      onClick={on_open_profile}
      size="comfort"
    >
      <WorkspaceIconFrame
        class_name="mx-auto h-14 w-14 overflow-hidden transition-all duration-300 hover:scale-110 hover:shadow-lg relative z-10"
        shape="round"
        size="lg"
      >
        {get_icon_avatar_src(agent.avatar) ? (
          <img
            alt={agent.name}
            className="h-full w-full object-cover transition-transform duration-300 hover:scale-105 hover:rotate-3"
            src={get_icon_avatar_src(agent.avatar) ?? undefined}
          />
        ) : (
          <Bot className="h-6 w-6 text-(--icon-strong) transition-transform duration-300 hover:scale-110 hover:rotate-6" />
        )}
      </WorkspaceIconFrame>

      <WorkspaceCatalogBody class_name="mt-3 w-full" grow={false}>
        <WorkspaceCatalogTitle size="lg" truncate>
          {agent.name}
        </WorkspaceCatalogTitle>

        {/* Agent 描述 */}
        {agent.description && (
          <WorkspaceCatalogDescription class_name="mt-1.5 line-clamp-2 text-[13px] leading-tight" min_height={false}>
            {agent.description}
          </WorkspaceCatalogDescription>
        )}

        {/* 运行配置信息 */}
        <div className="mt-2 flex flex-col gap-1 text-[11px] text-(--text-soft) items-center justify-center text-center">
          <div className="flex flex-wrap gap-1.5">
            <span className="text-(--text-default)">权限:</span>
            <span className="text-(--text-muted)">{permissionMode}</span>
          </div>
          <div className="flex flex-wrap gap-1.5 items-center justify-center">
            <span className="text-(--text-default)">Provider:</span>
            <span className="text-(--text-muted)">{provider}</span>
            <span className="mx-0.5">•</span>
            <span className="text-(--text-default)">工具:</span>
            <span className="text-(--text-muted)">{allowedToolsCount}</span>
            <span className="mx-0.5">•</span>
            <span className="text-(--text-default)">Skill:</span>
            <span className="text-(--text-muted)">{skillsCount}</span>
          </div>
        </div>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter class_name="mt-2 w-full gap-4" justify="center" onClick={(e) => e.stopPropagation()}>
        <WorkspaceCatalogTextAction onClick={on_open_room} tone="primary">
          <MessageSquareText className="h-3 w-3" />
          {t("contacts.chat")}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={on_create_team}>
          <Users className="h-3 w-3" />
          {t("contacts.create_team")}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
