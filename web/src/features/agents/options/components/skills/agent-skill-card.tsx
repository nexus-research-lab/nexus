import { Loader2, Lock } from "lucide-react";

import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

type SkillActionKind = "add" | "installed";

interface AgentSkillCardProps {
  actionKind: SkillActionKind;
  actionLabel: string;
  busy: boolean;
  commandBusy: boolean;
  onAction: (skill: AgentSkillEntry) => void;
  skill: AgentSkillEntry;
}

const ACTION_TONE = {
  add: "primary",
  installed: "default",
} as const;

export function AgentSkillCard({
  actionKind,
  actionLabel,
  busy,
  commandBusy,
  onAction,
  skill,
}: AgentSkillCardProps) {
  const { t } = useI18n();
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
      label: t("agent_options.skills.agent_workspace_only"),
      tone: "warning" as const,
      visible: skill.source_type === "workspace",
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
          <span className="min-w-0 text-[13.5px] font-semibold leading-[1.4] text-(--text-strong)">
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
        {skill.description ? (
          <p className="mt-1.5 line-clamp-2 text-[12px] leading-[1.55] text-(--text-muted)">
            {skill.description}
          </p>
        ) : null}
      </div>

      {skill.locked ? (
        <UiBadge className="shrink-0 self-start sm:mt-auto sm:mb-auto" size="xs" tone="success">
          {t("agent_options.skills.enabled")}
        </UiBadge>
      ) : (
        <UiButton
          className="shrink-0 self-end sm:mt-auto sm:mb-auto sm:self-auto"
          disabled={commandBusy}
          onClick={() => onAction(skill)}
          size="sm"
          tone={ACTION_TONE[actionKind]}
          type="button"
          variant="surface"
        >
          {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : actionLabel}
        </UiButton>
      )}
    </div>
  );
}
