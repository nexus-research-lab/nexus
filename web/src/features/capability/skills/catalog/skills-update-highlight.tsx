"use client";

import { AlertTriangle, CheckCircle2, Clock3, Loader2, RefreshCw } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { WORKSPACE_CATALOG_GRID_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import type { SkillInfo } from "@/types/capability/skill";

import type { SkillUpdateCheckNotice } from "../controller/skill-update-check-model";
import { SkillDirectoryCard } from "../shared/skill-directory-card";
import {
  buildSkillsUpdateModel,
  type SkillUpdateStatus,
} from "./skills-catalog-model";

interface SkillsUpdateHighlightProps {
  busySkillNames: ReadonlySet<string>;
  checkUpdateNotice: SkillUpdateCheckNotice | null;
  checkingUpdates: boolean;
  lastUpdateCheckedAt: number | null;
  onCheckUpdates: () => void;
  onOpenSkill: (skillName: string) => void;
  onUpdateSkill: (skillName: string) => void;
  updates: SkillInfo[];
}

const SKILL_UPDATE_STATUS_ICON = {
  checking: {
    className: "animate-spin",
    icon: Loader2,
  },
  current: {
    className: "text-(--success)",
    icon: CheckCircle2,
  },
  failure: {
    className: "text-(--destructive)",
    icon: AlertTriangle,
  },
  updates: {
    className: null,
    icon: Clock3,
  },
} satisfies Record<SkillUpdateStatus, {
  className: string | null;
  icon: typeof Clock3;
}>;

const SKILL_UPDATE_STATUS_SURFACE = {
  checking:
    "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-raised-background)_58%,transparent)]",
  current:
    "border-[color:color-mix(in_srgb,var(--success)_20%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_4%,transparent)]",
  failure:
    "border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_4%,transparent)]",
  updates:
    "border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)]",
} satisfies Record<SkillUpdateStatus, string>;

function SkillUpdateStatusIcon({ status }: { status: SkillUpdateStatus }) {
  const presentation = SKILL_UPDATE_STATUS_ICON[status];
  const Icon = presentation.icon;
  return <Icon className={cn("h-3.5 w-3.5", presentation.className)} />;
}

function UpdateSkillCard({
  busy,
  onOpen,
  onUpdate,
  skill,
}: {
  busy: boolean;
  onOpen: () => void;
  onUpdate: () => void;
  skill: SkillInfo;
}) {
  const { t } = useI18n();
  return (
    <SkillDirectoryCard
      action={(
        <UiButton
          className="pointer-events-auto"
          disabled={busy}
          onClick={onUpdate}
          size="sm"
          tone="primary"
          type="button"
          variant="solid"
        >
          {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {t("capability.skills_update")}
        </UiButton>
      )}
      badges={(
        <UiBadge size="xs" tone="warning">
          {t("capability.skills_update_available")}
        </UiBadge>
      )}
      busy={busy}
      description={getSkillDisplayDescription(skill, t)}
      meta={(
        <>
          <span className="truncate">
            {skill.source_name || t("capability.skills_external_import")}
          </span>
          <span className="shrink-0">· {skill.version || "unknown"}</span>
        </>
      )}
      onSelect={onOpen}
      seed={skill.name}
      title={skill.title || skill.name}
    />
  );
}

export function SkillsUpdateHighlight({
  busySkillNames,
  checkUpdateNotice,
  checkingUpdates,
  lastUpdateCheckedAt,
  onCheckUpdates,
  onOpenSkill,
  onUpdateSkill,
  updates,
}: SkillsUpdateHighlightProps) {
  const { locale, t } = useI18n();
  const model = buildSkillsUpdateModel({
    checkingUpdates,
    checkUpdateNotice,
    lastUpdateCheckedAt,
    updateCount: updates.length,
  }, { locale, t });
  if (!model) {
    return null;
  }
  const ActionIcon = model.actionDisabled ? Loader2 : RefreshCw;

  return (
    <section
      className={cn(
        "mb-5 rounded-[10px] border px-3 py-3",
        SKILL_UPDATE_STATUS_SURFACE[model.status],
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-md font-semibold tracking-[-0.025em] text-(--text-strong)">
              {model.title}
            </h2>
            {model.badgeLabel ? <UiBadge tone="warning">{model.badgeLabel}</UiBadge> : null}
          </div>
          <div className="mt-1 flex items-center gap-1.5 text-xs text-(--text-muted)">
            <SkillUpdateStatusIcon status={model.status} />
            <span>{model.statusLabel}</span>
          </div>
        </div>
        <UiButton
          disabled={model.actionDisabled}
          onClick={onCheckUpdates}
          size="sm"
          tone="primary"
          type="button"
          variant="surface"
        >
          <ActionIcon className={cn(
            "h-3.5 w-3.5",
            model.actionDisabled && "animate-spin",
          )} />
          {model.actionLabel}
        </UiButton>
      </div>

      {model.showUpdates ? (
        <div className={cn(WORKSPACE_CATALOG_GRID_CLASS_NAME, "mt-3 gap-2.5")}>
          {updates.map((skill) => (
            <UpdateSkillCard
              key={skill.name}
              busy={busySkillNames.has(skill.name)}
              onOpen={() => onOpenSkill(skill.name)}
              onUpdate={() => onUpdateSkill(skill.name)}
              skill={skill}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}
