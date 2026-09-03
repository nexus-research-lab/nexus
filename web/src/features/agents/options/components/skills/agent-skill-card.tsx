/**
 * INPUT: Agent Skill 条目、忙碌状态与启停命令。
 * OUTPUT: 本地化名称、用途摘要、必要来源徽标和开关组成的能力卡片。
 * POS: Agent 详情技能选择项；用途说明直接支持启停决策。
 */
import { Loader2, Lock } from "lucide-react";

import {
  getSkillDisplayDescription,
  getSkillDisplayTitle,
} from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type { AgentSkillEntry } from "@/types/capability/skill";

interface AgentSkillCardProps {
  actionLabel: string;
  blocked: boolean;
  busy: boolean;
  commandBusy: boolean;
  onAction: (skill: AgentSkillEntry) => void;
  skill: AgentSkillEntry;
}

function isAgentWorkspaceSource(skill: AgentSkillEntry): boolean {
  return skill.source_type === "workspace"
    || skill.storage_scope === "agent_workspace";
}

export function AgentSkillCard({
  actionLabel,
  blocked,
  busy,
  commandBusy,
  onAction,
  skill,
}: AgentSkillCardProps) {
  const { t } = useI18n();
  const title = getSkillDisplayTitle(skill, t);
  const description = getSkillDisplayDescription(skill, t);
  const badges = [
    {
      icon: <Lock className="h-3 w-3" />,
      key: "system",
      label: t("agent_options.skills.system_builtin"),
      tone: "success" as const,
      visible: skill.source_type === "system",
    },
    {
      key: "workspace",
      label: t("agent_options.skills.agent_workspace_local"),
      tone: "info" as const,
      visible: isAgentWorkspaceSource(skill),
    },
    {
      key: "main",
      label: t("agent_options.skills.main_only"),
      tone: "info" as const,
      visible: skill.scope === "main",
    },
  ].filter((badge) => badge.visible);

  return (
    <div className="grid min-h-[104px] grid-cols-[40px_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3.5 py-3 transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)">
      <UiSeededAvatar seed={skill.name} />
      <div className="flex min-h-10 min-w-0 items-center overflow-hidden">
        <div className="flex w-full min-w-0 flex-wrap items-center gap-2">
          <span className="line-clamp-2 min-w-0 text-sm font-semibold leading-[1.4] text-(--text-strong)">
            {title}
          </span>
          {badges.map((badge) => (
            <UiBadge
              className="shrink-0"
              icon={badge.icon}
              key={badge.key}
              size="xs"
              tone={badge.tone}
            >
              {badge.label}
            </UiBadge>
          ))}
        </div>
      </div>

      {!skill.locked ? (
        <div className="flex min-h-10 shrink-0 items-center gap-2">
          {busy ? (
            <Loader2
              className={getUiSpinnerClassName({ size: "sm", tone: "muted" })}
            />
          ) : null}
          <GlassSwitch
            aria-label={`${actionLabel} ${title}`}
            checked={skill.enabled_for_agent}
            disabled={commandBusy || blocked}
            onChange={() => onAction(skill)}
            size="xs"
          />
        </div>
      ) : null}

      {description ? (
        <p className="col-span-3 line-clamp-2 text-compact leading-[1.55] text-(--text-muted)">
          {description}
        </p>
      ) : null}
    </div>
  );
}
