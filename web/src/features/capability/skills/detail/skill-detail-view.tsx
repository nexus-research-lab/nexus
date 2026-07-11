"use client";

import {
  ArrowLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  Lock,
  Puzzle,
  RefreshCw,
  Trash2,
  type LucideIcon,
} from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiPanel } from "@/shared/ui/panel";

import {
  buildSkillDetailPresentation,
  getSkillDetailSnapshotTitle,
  type SkillDetailPresentation,
  type SkillDetailSnapshot,
} from "./skill-detail-model";
import { SkillMarkdown } from "./skill-markdown";

type SkillDetailAction = "delete" | "update";

interface SkillDetailViewProps {
  activeAction: SkillDetailAction | null;
  onBack: () => void;
  onDelete: () => void;
  onUpdate: () => void;
  snapshot: SkillDetailSnapshot;
}

const SKILL_ICON_MAP: Record<SkillDetailPresentation["icon"], LucideIcon> = {
  lock: Lock,
  puzzle: Puzzle,
};

export function SkillDetailView({
  activeAction,
  onBack,
  onDelete,
  onUpdate,
  snapshot,
}: SkillDetailViewProps) {
  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <SkillDetailBreadcrumb
        onBack={onBack}
        title={getSkillDetailSnapshotTitle(snapshot)}
      />
      <SkillDetailContent
        activeAction={activeAction}
        onBack={onBack}
        onDelete={onDelete}
        onUpdate={onUpdate}
        snapshot={snapshot}
      />
    </div>
  );
}

function SkillDetailBreadcrumb({
  onBack,
  title,
}: {
  onBack: () => void;
  title: string | null;
}) {
  return (
    <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
      <button
        className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
        onClick={onBack}
        type="button"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        技能
      </button>
      {title ? (
        <>
          <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
          <span className="truncate font-medium text-(--text-strong)">{title}</span>
        </>
      ) : null}
    </div>
  );
}

function SkillDetailContent({
  activeAction,
  onBack,
  onDelete,
  onUpdate,
  snapshot,
}: SkillDetailViewProps) {
  if (snapshot.status === "loading") {
    return (
      <UiStateBlock
        className="min-h-[420px]"
        icon={<Loader2 className="h-6 w-6 animate-spin" />}
        size="md"
        title="加载技能详情中..."
        variant="plain"
      />
    );
  }
  if (snapshot.status === "error") {
    return (
      <UiStateBlock
        actions={(
          <UiButton onClick={onBack} size="sm" type="button">
            返回技能
          </UiButton>
        )}
        className="min-h-[420px]"
        description={snapshot.errorMessage}
        size="md"
        title="技能不存在"
        tone="danger"
        variant="plain"
      />
    );
  }

  return (
    <SkillDetailReady
      activeAction={activeAction}
      model={buildSkillDetailPresentation(snapshot.skill)}
      onDelete={onDelete}
      onUpdate={onUpdate}
    />
  );
}

function SkillDetailReady({
  activeAction,
  model,
  onDelete,
  onUpdate,
}: {
  activeAction: SkillDetailAction | null;
  model: SkillDetailPresentation;
  onDelete: () => void;
  onUpdate: () => void;
}) {
  return (
    <div className="pt-9">
      <SkillDetailHero
        activeAction={activeAction}
        model={model}
        onDelete={onDelete}
        onUpdate={onUpdate}
      />
      <div className="mt-8 space-y-6">
        <SkillDetailBadges badges={model.badges} />
        <section>
          <h2 className="mb-3 text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
            技能说明
          </h2>
          <UiPanel padding="md" radius="md" variant="inset">
            <SkillMarkdown
              description={model.description}
              markdown={model.readmeMarkdown}
              title={model.displayName}
            />
          </UiPanel>
        </section>
        <SkillSourceLink sourceUrl={model.sourceUrl} />
      </div>
    </div>
  );
}

function SkillDetailHero({
  activeAction,
  model,
  onDelete,
  onUpdate,
}: {
  activeAction: SkillDetailAction | null;
  model: SkillDetailPresentation;
  onDelete: () => void;
  onUpdate: () => void;
}) {
  const SkillIcon = SKILL_ICON_MAP[model.icon];

  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-4">
          <div
            className={cn(
              "flex h-16 w-16 shrink-0 items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-(--surface-panel-background) shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
              model.iconClassName,
            )}
          >
            <SkillIcon className="h-9 w-9" />
          </div>
          <h1 className="min-w-0 text-[24px] font-semibold tracking-[-0.035em] text-(--text-strong)">
            <span className="truncate">{model.displayName}</span>{" "}
            <span className="font-normal text-(--text-muted)">Skill</span>
          </h1>
        </div>
        <p className="mt-4 text-[15px] leading-6 text-(--text-muted)">
          {model.description}
        </p>
      </div>
      <SkillDetailActions
        activeAction={activeAction}
        canDelete={model.canDelete}
        canUpdate={model.canUpdate}
        onDelete={onDelete}
        onUpdate={onUpdate}
      />
    </div>
  );
}

function SkillDetailActions({
  activeAction,
  canDelete,
  canUpdate,
  onDelete,
  onUpdate,
}: {
  activeAction: SkillDetailAction | null;
  canDelete: boolean;
  canUpdate: boolean;
  onDelete: () => void;
  onUpdate: () => void;
}) {
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
      <SkillUpdateButton
        activeAction={activeAction}
        onUpdate={onUpdate}
        visible={canUpdate}
      />
      <SkillDeleteButton
        activeAction={activeAction}
        onDelete={onDelete}
        visible={canDelete}
      />
    </div>
  );
}

function SkillUpdateButton({
  activeAction,
  onUpdate,
  visible,
}: {
  activeAction: SkillDetailAction | null;
  onUpdate: () => void;
  visible: boolean;
}) {
  if (!visible) return null;
  const updating = activeAction === "update";

  return (
    <UiButton
      disabled={activeAction !== null}
      onClick={onUpdate}
      size="sm"
      tone="primary"
      type="button"
      variant="solid"
    >
      {updating
        ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
        : <RefreshCw className="h-3.5 w-3.5" />}
      更新技能
    </UiButton>
  );
}

function SkillDeleteButton({
  activeAction,
  onDelete,
  visible,
}: {
  activeAction: SkillDetailAction | null;
  onDelete: () => void;
  visible: boolean;
}) {
  if (!visible) return null;

  return (
    <UiButton
      disabled={activeAction !== null}
      onClick={onDelete}
      size="sm"
      tone="danger"
      type="button"
      variant="surface"
    >
      <Trash2 className="h-3.5 w-3.5" />
      {activeAction === "delete" ? "删除中" : "删除"}
    </UiButton>
  );
}

function SkillDetailBadges({
  badges,
}: {
  badges: SkillDetailPresentation["badges"];
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {badges.map((badge) => (
        <UiBadge key={badge.key} tone={badge.tone}>
          {badge.label}
        </UiBadge>
      ))}
    </div>
  );
}

function SkillSourceLink({ sourceUrl }: { sourceUrl: string | null }) {
  if (!sourceUrl) return null;

  return (
    <a
      className="inline-flex items-center gap-2 text-[13px] font-semibold text-(--primary) underline decoration-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] underline-offset-4"
      href={sourceUrl}
      rel="noopener noreferrer"
      target="_blank"
    >
      <ExternalLink className="h-3.5 w-3.5" />
      查看来源
    </a>
  );
}
