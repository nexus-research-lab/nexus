// INPUT: Skill 更新检查投影、可更新条目与检查/打开/更新动作。
// OUTPUT: 共享 Panel 中的更新状态、可键盘操作的 Skill 列表和统一加载反馈。
// POS: Skill 目录更新摘要纯视图；检查与写命令生命周期归 controller。
"use client";

import { AlertTriangle, CheckCircle2, Clock3, Loader2, RefreshCw } from "lucide-react";

import { getSkillDisplayDescription } from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
    className: null,
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
    "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_58%,transparent)]",
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
      className={status === "checking"
        ? getUiSpinnerClassName({ size: "sm" })
        : cn("h-3.5 w-3.5 shrink-0", presentation.className)}
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
    <UiListRow
      aria-label={title}
      aria-busy={busy || undefined}
      className="grid min-w-0 grid-cols-[32px_minmax(0,1fr)_auto] py-2.5"
      density="compact"
      leading={<UiSeededAvatar seed={skill.name} size="xs" />}
      muted={busy}
      variant="outlined"
      onClick={onOpen}
      right={(
        <UiButton
          className="shrink-0"
          disabled={busy}
          onClick={(event) => {
            event.stopPropagation();
            onUpdate();
          }}
          size="sm"
          tone="primary"
          type="button"
          variant="solid"
        >
          {busy ? (
            <Loader2
              className={getUiSpinnerClassName({ size: "sm" })}
              strokeWidth={1.8}
            />
          ) : null}
          {t("capability.skills_update")}
        </UiButton>
      )}
    >
      <div className="min-w-0">
        <div className="flex min-w-0 items-baseline gap-2">
          <h3 className={cn(
            "truncate",
            getUiTypographyClassName({
              role: "control",
              tone: "strong",
              weight: "semibold",
            }),
          )}>
            {title}
          </h3>
          <span className={cn(
            "min-w-0 truncate",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            {skill.source_name || t("capability.skills_external_import")} · {skill.version || "unknown"}
          </span>
        </div>
        {description ? (
          <p className={cn(
            "mt-0.5 truncate",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {description}
          </p>
        ) : null}
      </div>
    </UiListRow>
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
    <UiPanel
      className={cn(
        "mb-5",
        SKILL_UPDATE_STATUS_SURFACE[model.status],
      )}
      padding="sm"
      radius="md"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <h2 className={getUiTypographyClassName({
            role: "sectionTitle",
            tone: "strong",
            weight: "semibold",
          })}>
            {model.title}
          </h2>
          {model.badgeLabel ? <UiBadge tone="warning">{model.badgeLabel}</UiBadge> : null}
          <span className={cn(
            "flex min-w-0 items-center gap-1.5",
            getUiTypographyClassName({ role: "caption", tone: "muted" }),
          )}>
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
          <ActionIcon
            className={model.actionDisabled
              ? getUiSpinnerClassName({ size: "sm" })
              : "h-3.5 w-3.5"}
            strokeWidth={1.8}
          />
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
    </UiPanel>
  );
}
