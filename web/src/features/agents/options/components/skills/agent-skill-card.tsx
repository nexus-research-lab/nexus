import { Loader2, Lock } from "lucide-react";

import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
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
    <div className="flex h-[92px] items-start justify-between gap-3 rounded-[8px] border border-(--divider-subtle-color) bg-transparent px-3 py-2.5 transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)">
      <div className="min-w-0 flex-1 overflow-hidden">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="min-w-0 truncate text-[12.5px] font-semibold leading-[1.35] text-(--text-strong)">
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
          <p className="mt-0.5 line-clamp-2 text-[11.5px] leading-normal text-(--text-muted)">
            {skill.description}
          </p>
        ) : null}
      </div>

      {skill.locked ? (
        <UiBadge className="mt-auto mb-auto shrink-0" size="xs" tone="success">
          {t("agent_options.skills.enabled")}
        </UiBadge>
      ) : (
        <UiButton
          className="mt-auto mb-auto shrink-0"
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
