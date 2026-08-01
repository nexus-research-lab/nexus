import { Loader2, Lock } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type { AgentSkillEntry } from "@/types/capability/skill";

interface AgentSkillCardProps {
  actionLabel: string;
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
  busy,
  commandBusy,
  onAction,
  skill,
}: AgentSkillCardProps) {
  const { t } = useI18n();
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
    <div className="flex min-h-[108px] flex-col items-stretch justify-between gap-3 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-4 py-3.5 transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) sm:flex-row sm:items-start sm:gap-4">
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="min-w-0 text-sm font-semibold leading-[1.4] text-(--text-strong)">
            {skill.title || skill.name}
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
        {description ? (
          <p className="mt-1.5 line-clamp-2 text-compact leading-[1.55] text-(--text-muted)">
            {description}
          </p>
        ) : null}
      </div>

      {!skill.locked ? (
        <div className="flex min-h-7 shrink-0 items-center gap-2 self-end sm:mt-auto sm:mb-auto sm:self-auto">
          {busy ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin text-(--text-muted)" />
          ) : null}
          <GlassSwitch
            aria-label={`${actionLabel} ${skill.title || skill.name}`}
            checked={skill.enabled_for_agent}
            disabled={commandBusy}
            onChange={() => onAction(skill)}
            size="xs"
          />
        </div>
      ) : null}
    </div>
  );
}
