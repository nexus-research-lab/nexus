"use client";

import { AlertTriangle, CheckCircle2, Clock3, Loader2, Puzzle, RefreshCw } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { UiListRow } from "@/shared/ui/list-row";
import type { SkillInfo } from "@/types/capability/skill";

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

function formatCheckedTime(value: number | null): string {
  if (!value) return "尚未检查";
  return new Date(value).toLocaleString("zh-CN", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit",
  });
}

function statusLabel(
  checkingUpdates: boolean,
  checkUpdateMessage: string | null,
  lastUpdateCheckedAt: number | null,
): string {
  if (checkingUpdates) return "正在检查远端版本...";
  if (checkUpdateMessage) return checkUpdateMessage;
  return `上次检查 ${formatCheckedTime(lastUpdateCheckedAt)}`;
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
  const hasFailure = checkUpdateMessage?.includes("无法检查") ?? false;
  const shouldShow = checkingUpdates || Boolean(checkUpdateMessage) || updates.length > 0;

  if (!shouldShow) return null;

  return (
    <section className="mb-7 rounded-[16px] border border-[color:color-mix(in_srgb,var(--warning)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)] px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
              {updates.length ? "可更新 Skill" : "更新检查"}
            </h2>
            {updates.length ? <UiBadge tone="warning">{updates.length} 个可更新</UiBadge> : null}
          </div>
          <div className="mt-1 flex items-center gap-1.5 text-xs text-(--text-muted)">
            {checkingUpdates ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : hasFailure ? (
              <AlertTriangle className="h-3.5 w-3.5 text-(--destructive)" />
            ) : updates.length ? (
              <Clock3 className="h-3.5 w-3.5" />
            ) : (
              <CheckCircle2 className="h-3.5 w-3.5 text-(--success)" />
            )}
            <span>{statusLabel(checkingUpdates, checkUpdateMessage, lastUpdateCheckedAt)}</span>
          </div>
        </div>
        <UiButton
          disabled={checkingUpdates}
          onClick={onCheckUpdates}
          size="sm"
          tone="primary"
          type="button"
          variant="surface"
        >
          {checkingUpdates ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {checkingUpdates ? "检查中" : "重新检查"}
        </UiButton>
      </div>

      {updates.length ? (
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
