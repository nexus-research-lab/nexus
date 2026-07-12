"use client";

import { AlertTriangle, CheckCircle2, Clock3, Loader2, Puzzle, RefreshCw } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { SkillInfo } from "@/types/capability/skill";

import {
  buildSkillsUpdateModel,
  type SkillUpdateStatus,
} from "./skills-catalog-model";

interface SkillsUpdateHighlightProps {
  busySkillNames: ReadonlySet<string>;
  checkUpdateMessage: string | null;
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

function SkillUpdateStatusIcon({ status }: { status: SkillUpdateStatus }) {
  const presentation = SKILL_UPDATE_STATUS_ICON[status];
  const Icon = presentation.icon;
  return <Icon className={cn("h-3.5 w-3.5", presentation.className)} />;
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
  return (
    <UiListRow
      className={cn("min-h-[74px] rounded-[12px] px-2 py-2", busy && "opacity-70")}
      leading={(
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--status-info-soft-text)">
          <Puzzle className="h-4 w-4" />
        </span>
      )}
      onClick={onOpen}
      right={(
        <UiButton
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
          {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          更新
        </UiButton>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold text-(--text-strong)">
            {skill.title || skill.name}
          </span>
          <UiBadge size="xs" tone="warning">有更新</UiBadge>
        </div>
        <p className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {skill.description || "暂无描述"}
        </p>
        <p className="mt-0.5 truncate text-[11px] leading-4 text-(--text-soft)">
          {skill.source_name || "外部导入"} · {skill.version || "unknown"}
        </p>
      </div>
    </UiListRow>
  );
}

export function SkillsUpdateHighlight({
  busySkillNames,
  checkUpdateMessage,
  checkingUpdates,
  lastUpdateCheckedAt,
  onCheckUpdates,
  onOpenSkill,
  onUpdateSkill,
  updates,
}: SkillsUpdateHighlightProps) {
  const model = buildSkillsUpdateModel({
    checkingUpdates,
    checkUpdateMessage,
    lastUpdateCheckedAt,
    updateCount: updates.length,
  });
  if (!model) {
    return null;
  }
  const ActionIcon = model.actionDisabled ? Loader2 : RefreshCw;

  return (
    <section className="mb-7 rounded-[16px] border border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)] px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
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
        <div className="mt-3 grid grid-cols-1 gap-2 md:grid-cols-2">
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
