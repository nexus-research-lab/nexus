/**
 * INPUT: Skill 详情快照、Agent 使用矩阵与更新删除命令。
 * OUTPUT: Skill 身份、范围说明、Agent 状态差异和完整正文详情。
 * POS: Skill 详情纯视图；开关附近保留影响当前决策的说明。
 */
"use client";

import {
  ArrowLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  RefreshCw,
  Trash2,
} from "lucide-react";

import {
  getSkillDisplayDescription,
  getSkillDisplayTitle,
} from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { WorkspaceContentDetailHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiPanel } from "@/shared/ui/panel";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import type { SkillAgentBinding } from "@/types/capability/skill";

import {
  buildSkillAgentBindingPresentation,
  buildSkillDetailPresentation,
  getSkillDetailSnapshotTitle,
  type SkillAgentBindingsReadFailure,
  type SkillAgentToggleFailure,
  type SkillDetailPresentation,
  type SkillDetailSnapshot,
} from "./skill-detail-model";
import { SkillMarkdown } from "./skill-markdown";

type SkillDetailAction = "delete" | "update" | "toggle";

interface SkillDetailViewProps {
  activeAction: SkillDetailAction | null;
  agentBindings: SkillAgentBinding[];
  agentsLoading: boolean;
  bindingsFailure: SkillAgentBindingsReadFailure | null;
  busyAgentId: string | null;
  onBack: () => void;
  onAgentToggle: (binding: SkillAgentBinding) => void;
  onDelete: () => void;
  onRetry: () => void;
  onRetryBindings: () => void;
  onUpdate: () => void;
  snapshot: SkillDetailSnapshot;
  toggleFailures: Readonly<Record<string, SkillAgentToggleFailure>>;
}

export function SkillDetailView({
  activeAction,
  agentBindings,
  agentsLoading,
  bindingsFailure,
  busyAgentId,
  onBack,
  onAgentToggle,
  onDelete,
  onRetry,
  onRetryBindings,
  onUpdate,
  snapshot,
  toggleFailures,
}: SkillDetailViewProps) {
  return (
    <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
      <SkillDetailBreadcrumb
        onBack={onBack}
        title={getSkillDetailSnapshotTitle(snapshot)}
      />
      <SkillDetailContent
        activeAction={activeAction}
        agentBindings={agentBindings}
        agentsLoading={agentsLoading}
        bindingsFailure={bindingsFailure}
        busyAgentId={busyAgentId}
        onAgentToggle={onAgentToggle}
        onDelete={onDelete}
        onRetry={onRetry}
        onRetryBindings={onRetryBindings}
        onUpdate={onUpdate}
        snapshot={snapshot}
        toggleFailures={toggleFailures}
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
  const { t } = useI18n();
  return (
    <WorkspaceContentDetailHeader>
      <div className="flex min-w-0 items-center gap-2 text-sm text-(--text-muted)">
        <button
          className="inline-flex items-center gap-1 rounded-[8px] px-1.5 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
          onClick={onBack}
          type="button"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          {t("capability.skills_detail_back")}
        </button>
        {title ? (
          <>
            <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
            <span className="truncate font-medium text-(--text-strong)">{title}</span>
          </>
        ) : null}
      </div>
    </WorkspaceContentDetailHeader>
  );
}

function SkillDetailContent({
  activeAction,
  agentBindings,
  agentsLoading,
  bindingsFailure,
  busyAgentId,
  onAgentToggle,
  onDelete,
  onRetry,
  onRetryBindings,
  onUpdate,
  snapshot,
  toggleFailures,
}: Omit<SkillDetailViewProps, "onBack">) {
  const { t } = useI18n();
  if (snapshot.status === "loading") {
    return (
      <UiStateBlock
        className="min-h-[420px]"
        icon={<Loader2 className="h-6 w-6 animate-spin" />}
        size="md"
        title={t("capability.skills_detail_loading")}
        variant="plain"
      />
    );
  }
  if (snapshot.status === "error") {
    return (
      <UiResourceState
        className="min-h-[420px]"
        impact={t("state.read_failure_impact")}
        primaryAction={{
          label: t("state.retry"),
          onClick: onRetry,
        }}
        size="md"
        state="error"
        title={t("capability.skills_detail_load_failed")}
        variant="plain"
      />
    );
  }

  return (
    <SkillDetailReady
      activeAction={activeAction}
      agentBindings={agentBindings}
      agentsLoading={agentsLoading}
      bindingsFailure={bindingsFailure}
      busyAgentId={busyAgentId}
      model={{
        ...buildSkillDetailPresentation(
          snapshot.skill,
          getSkillDisplayDescription(snapshot.skill, t),
          { t },
        ),
        displayName: getSkillDisplayTitle(snapshot.skill, t),
      }}
      onAgentToggle={onAgentToggle}
      onDelete={onDelete}
      onUpdate={onUpdate}
      onRetryBindings={onRetryBindings}
      toggleFailures={toggleFailures}
    />
  );
}

function SkillDetailReady({
  activeAction,
  agentBindings,
  agentsLoading,
  bindingsFailure,
  busyAgentId,
  model,
  onAgentToggle,
  onDelete,
  onUpdate,
  onRetryBindings,
  toggleFailures,
}: {
  activeAction: SkillDetailAction | null;
  agentBindings: SkillAgentBinding[];
  agentsLoading: boolean;
  bindingsFailure: SkillAgentBindingsReadFailure | null;
  busyAgentId: string | null;
  model: SkillDetailPresentation;
  onAgentToggle: (binding: SkillAgentBinding) => void;
  onDelete: () => void;
  onUpdate: () => void;
  onRetryBindings: () => void;
  toggleFailures: Readonly<Record<string, SkillAgentToggleFailure>>;
}) {
  const { t } = useI18n();
  return (
    <div className="pt-5">
      <SkillDetailHero
        activeAction={activeAction}
        model={model}
        onDelete={onDelete}
        onUpdate={onUpdate}
      />
      <div className="mt-6 max-w-[760px] space-y-5">
        <SkillDetailBadges badges={model.badges} />
        {model.scope === "room" ? (
          <RoomSkillUsage />
        ) : (
          <SkillAgentBindings
            agentBindings={agentBindings}
            agentsLoading={agentsLoading}
            bindingsFailure={bindingsFailure}
            busyAgentId={busyAgentId}
            locked={model.locked}
            onToggle={onAgentToggle}
            onRetryBindings={onRetryBindings}
            toggleFailures={toggleFailures}
          />
        )}
        <section>
          <h2 className="mb-3 text-md font-semibold tracking-[-0.025em] text-(--text-strong)">
            {t("capability.skills_detail_description")}
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

function RoomSkillUsage() {
  const { t } = useI18n();
  return (
    <section>
      <h2 className="text-md font-semibold tracking-[-0.025em] text-(--text-strong)">
        {t("capability.skills_detail_room_scope")}
      </h2>
      <p className="mt-1 text-sm text-(--text-muted)">
        {t("capability.skills_detail_room_scope_description")}
      </p>
    </section>
  );
}

function SkillAgentBindings({
  agentBindings,
  agentsLoading,
  bindingsFailure,
  busyAgentId,
  locked,
  onRetryBindings,
  onToggle,
  toggleFailures,
}: {
  agentBindings: SkillAgentBinding[];
  agentsLoading: boolean;
  bindingsFailure: SkillAgentBindingsReadFailure | null;
  busyAgentId: string | null;
  locked: boolean;
  onRetryBindings: () => void;
  onToggle: (binding: SkillAgentBinding) => void;
  toggleFailures: Readonly<Record<string, SkillAgentToggleFailure>>;
}) {
  const { t } = useI18n();
  const enabledCount = agentBindings.filter((item) => item.enabled).length;
  return (
    <section>
      <div className="mb-3 flex items-end justify-between gap-3">
        <div>
          <h2 className="text-md font-semibold tracking-[-0.025em] text-(--text-strong)">
            {t("capability.skills_detail_agent_scope")}
          </h2>
          <p className="mt-1 text-sm text-(--text-muted)">
            {t("capability.skills_detail_agent_scope_description")}
          </p>
        </div>
        {!agentsLoading ? (
          <span className="text-xs text-(--text-soft)">
            {t("capability.skills_detail_enabled_count", {
              enabled: enabledCount,
              total: agentBindings.length,
            })}
          </span>
        ) : null}
      </div>
      <UiPanel padding="sm" radius="md" variant="inset">
        {bindingsFailure ? (
          <SkillAgentFailureNotice
            failure={bindingsFailure}
            onRefresh={onRetryBindings}
            refreshLabel={t("state.retry")}
          />
        ) : null}
        {agentsLoading ? (
          <div className="flex items-center gap-2 px-3 py-3 text-sm text-(--text-muted)">
            <Loader2 className="h-4 w-4 animate-spin" />
            {t("capability.skills_detail_bindings_loading")}
          </div>
        ) : agentBindings.length === 0 && !bindingsFailure ? (
          <p className="px-3 py-3 text-sm text-(--text-muted)">
            {t("capability.skills_detail_no_agents")}
          </p>
        ) : (
          <div className="divide-y divide-(--divider-subtle-color)">
            {agentBindings.map((binding) => {
              const presentation = buildSkillAgentBindingPresentation(
                binding,
                locked,
                t,
              );
              const failure = toggleFailures[binding.agent_id] ?? null;
              return (
                <div key={binding.agent_id}>
                  <div className="flex items-center justify-between gap-3 px-3 py-2.5">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-(--text-strong)">
                        {binding.agent_name}
                      </p>
                      <p className="text-xs text-(--text-soft)">
                        {presentation.description}
                      </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <span className="text-xs text-(--text-muted)">
                        {presentation.status}
                      </span>
                      <GlassSwitch
                        aria-label={presentation.switchLabel}
                        checked={binding.enabled}
                        disabled={
                          locked
                          || !binding.available
                          || busyAgentId !== null
                          || Boolean(failure?.blocksRepeat)
                        }
                        onChange={() => onToggle(binding)}
                        size="xs"
                      />
                    </div>
                  </div>
                  {failure ? (
                    <SkillAgentFailureNotice
                      className="mx-3 mb-3"
                      failure={failure}
                      onRefresh={failure.blocksRepeat ? onRetryBindings : undefined}
                      refreshLabel={t("state.reload_check")}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
        )}
      </UiPanel>
    </section>
  );
}

function SkillAgentFailureNotice({
  className = "mb-2",
  failure,
  onRefresh,
  refreshLabel,
}: {
  className?: string;
  failure: SkillAgentBindingsReadFailure | SkillAgentToggleFailure;
  onRefresh?: () => void;
  refreshLabel: string;
}) {
  return (
    <div
      aria-live="polite"
      className={`${className} rounded-[8px] border px-3 py-2 ${failure.tone === "warning" ? "border-[color:color-mix(in_srgb,var(--warning)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_5%,transparent)]" : "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] bg-(--status-danger-soft-background)"}`}
      role="status"
    >
      <p className="text-xs font-semibold text-(--text-strong)">
        {failure.title}
      </p>
      <RecoverySummary className="mt-0.5" impact={failure.impact} />
      {onRefresh ? (
        <UiButton className="mt-1" onClick={onRefresh} size="xs" type="button" variant="text">
          {refreshLabel}
        </UiButton>
      ) : null}
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
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-4">
          <UiSeededAvatar seed={model.avatarSeed} size="lg" />
          <h1 className="min-w-0 text-lg font-semibold tracking-[-0.025em] text-(--text-strong)">
            <span className="truncate">{model.displayName}</span>
          </h1>
        </div>
        {model.description ? (
          <p className="mt-3 text-sm leading-5 text-(--text-muted)">
            {model.description}
          </p>
        ) : null}
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
  const { t } = useI18n();
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
      {t("capability.skills_detail_update")}
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
  const { t } = useI18n();
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
      {activeAction === "delete"
        ? t("capability.skills_detail_deleting")
        : t("capability.skills_detail_delete")}
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
  const { t } = useI18n();
  if (!sourceUrl) return null;

  return (
    <a
      className="inline-flex items-center gap-2 text-sm font-semibold text-(--primary) underline decoration-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] underline-offset-4"
      href={sourceUrl}
      rel="noopener noreferrer"
      target="_blank"
    >
      <ExternalLink className="h-3.5 w-3.5" />
      {t("capability.skills_detail_view_source")}
    </a>
  );
}
