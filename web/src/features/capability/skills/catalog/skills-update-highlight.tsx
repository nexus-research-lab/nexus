"use client";

import { AlertTriangle, CheckCircle2, Clock3, Loader2, RefreshCw } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import type { SkillInfo } from "@/types/capability/skill";

import type { SkillUpdateCheckNotice } from "../controller/skill-update-check-model";
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
  return (
    <Icon
      aria-hidden="true"
      className={cn("h-3.5 w-3.5 shrink-0", presentation.className)}
      strokeWidth={1.8}
    />
  );
}

function UpdateSkillRow({
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
  const description = getSkillDisplayDescription(skill, t);
  const title = skill.title || skill.name;
  return (
    <article
      aria-busy={busy || undefined}
      className={cn(
        "relative grid min-w-0 grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-3 rounded-[8px] border border-(--divider-subtle-color) bg-transparent px-3 py-2.5 transition duration-(--motion-duration-fast) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        busy && "opacity-70",
      )}
    >
      <button
        aria-label={title}
        className="absolute inset-0 rounded-[inherit] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
        onClick={onOpen}
        type="button"
      />
      <UiSeededAvatar
        className="pointer-events-none relative z-10"
        seed={skill.name}
        size="xs"
      />
      <div className="pointer-events-none relative z-10 min-w-0">
        <div className="flex min-w-0 items-baseline gap-2">
          <h3 className="truncate text-sm font-semibold text-(--text-strong)">
            {title}
          </h3>
          <span className="min-w-0 truncate text-2xs text-(--text-soft)">
            {skill.source_name || t("capability.skills_external_import")} · {skill.version || "unknown"}
          </span>
        </div>
        {description ? (
          <p className="mt-0.5 truncate text-compact leading-[1.125rem] text-(--text-muted)">
            {description}
          </p>
        ) : null}
      </div>
      <UiButton
        className="relative z-10 shrink-0"
        disabled={busy}
        onClick={onUpdate}
        size="sm"
        tone="primary"
        type="button"
        variant="solid"
      >
        {busy ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" strokeWidth={1.8} />
        ) : null}
        {t("capability.skills_update")}
      </UiButton>
    </article>
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
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <h2 className="text-md font-semibold tracking-[-0.025em] text-(--text-strong)">
            {model.title}
          </h2>
          {model.badgeLabel ? <UiBadge tone="warning">{model.badgeLabel}</UiBadge> : null}
          <span className="flex min-w-0 items-center gap-1.5 text-xs text-(--text-muted)">
            <SkillUpdateStatusIcon status={model.status} />
            <span className="truncate">{model.statusLabel}</span>
          </span>
        </div>
        <UiButton
          disabled={model.actionDisabled}
          onClick={onCheckUpdates}
          size="sm"
          tone="default"
          type="button"
          variant="ghost"
        >
          <ActionIcon className={cn(
            "h-3.5 w-3.5",
            model.actionDisabled && "animate-spin",
          )} strokeWidth={1.8} />
          {model.actionLabel}
        </UiButton>
      </div>

      {model.showUpdates ? (
        <div className="mt-3 grid gap-2">
          {updates.map((skill) => (
            <UpdateSkillRow
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
