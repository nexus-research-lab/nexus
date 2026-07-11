"use client";

import { MessageSquareText, Users } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { Agent } from "@/types/agent/agent";
import { formatProviderLabel } from "@/types/capability/provider";
import { WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-content";

interface ContactsAgentCardProps {
  agent: Agent;
  /** 点击卡片本身 → 打开 AgentOptions 对话框（edit 模式） */
  onOpenProfile: () => void;
  /** 💬 Chat 按钮 → ensureDirectRoom 发起 DM */
  onOpenRoom: () => void;
  /** 👥 Create Team 按钮 → 用该 Agent 创建 Room */
  onCreateTeam: () => void;
}

/** Agent 卡片 — 居中布局，底部动作收为轻量文本按钮，避免主区继续堆胶囊层。 */
export function ContactsAgentCard({
  agent,
  onOpenProfile: onOpenProfile,
  onOpenRoom: onOpenRoom,
  onCreateTeam: onCreateTeam,
}: ContactsAgentCardProps) {
  const { t } = useI18n();

  // 提取配置信息
  const permissionMode = agent.options.permission_mode || "default";
  const provider = formatProviderLabel(agent.options.provider);
  const allowedToolsCount = agent.options.allowed_tools?.length || 0;
  const skillsCount = agent.skills_count || 0;

  return (
    <WorkspaceCatalogCard
      align="center"
      className="relative h-full overflow-hidden"
      interactive
      onClick={onOpenProfile}
      size="comfort"
    >
      <UiAgentAvatar
        avatar={agent.avatar}
        className="relative z-10 mx-auto transition-all duration-300 hover:scale-105"
        name={agent.name}
        size="lg"
      />

      <WorkspaceCatalogBody className="mt-3 w-full" grow={false}>
        <WorkspaceCatalogTitle size="lg" truncate>
          {agent.name}
        </WorkspaceCatalogTitle>

        {/* Agent 描述 */}
        {agent.description && (
          <WorkspaceCatalogDescription className="mt-1.5 line-clamp-2 text-[13px] leading-tight" minHeight={false}>
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

      <WorkspaceCatalogFooter className="mt-2 w-full gap-4" justify="center" onClick={(e) => e.stopPropagation()}>
        <WorkspaceCatalogTextAction onClick={onOpenRoom} tone="primary">
          <MessageSquareText className="h-3 w-3" />
          {t("contacts.chat")}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={onCreateTeam}>
          <Users className="h-3 w-3" />
          {t("contacts.create_team")}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
