/**
 * INPUT: Skill 详情快照、Agent 使用矩阵与更新删除命令。
 * OUTPUT: 共享对象身份区中的 Skill 信息、正文阅读列、Agent 配置侧栏及响应式单列详情。
 * POS: Skill 详情纯视图；复用 Capability 身份与详情分栏，不拥有页面断点或列宽。
 */
"use client";

import {
  ExternalLink,
  Loader2,
  RefreshCw,
  Trash2,
} from "lucide-react";

import {
  CapabilityDetailIdentity,
  CapabilityDetailPage,
  CapabilityDetailSectionHeader,
  CapabilityDetailSplitLayout,
} from "@/features/capability/shared/capability-page-layout";
import {
  getSkillDisplayDescription,
  getSkillDisplayTitle,
} from "@/lib/skill-description";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiPanel } from "@/shared/ui/panel";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  const { t } = useI18n();
  return (
    <CapabilityDetailPage
      backLabel={t("capability.skills_detail_back")}
      currentTitle={getSkillDetailSnapshotTitle(snapshot) ?? undefined}
      onBack={onBack}
    >
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
    </CapabilityDetailPage>
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
      <CapabilityDetailSplitLayout
        aside={(
          <div className="space-y-5">
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
          </div>
        )}
        header={(
          <SkillDetailHero
            activeAction={activeAction}
            model={model}
            onDelete={onDelete}
            onUpdate={onUpdate}
          />
        )}
      >
        <div className="space-y-5">
          <section>
            <CapabilityDetailSectionHeader
              title={t("capability.skills_detail_description")}
            />
            <UiPanel padding="md" radius="md" variant="card">
              <SkillMarkdown
                description={model.description}
                markdown={model.readmeMarkdown}
                title={model.displayName}
              />
            </UiPanel>
          </section>
          <SkillSourceLink sourceUrl={model.sourceUrl} />
        </div>
      </CapabilityDetailSplitLayout>
    </div>
  );
}

function RoomSkillUsage() {
  const { t } = useI18n();
  return (
    <section>
      <CapabilityDetailSectionHeader
        description={t("capability.skills_detail_room_scope_description")}
        title={t("capability.skills_detail_room_scope")}
      />
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
      <CapabilityDetailSectionHeader
        description={t("capability.skills_detail_agent_scope_description")}
        meta={!agentsLoading
          ? t("capability.skills_detail_enabled_count", {
              enabled: enabledCount,
              total: agentBindings.length,
            })
          : undefined}
        title={t("capability.skills_detail_agent_scope")}
      />
      <UiPanel padding="sm" radius="md" variant="card">
        {bindingsFailure ? (
          <SkillAgentFailureNotice
            failure={bindingsFailure}
            onRefresh={onRetryBindings}
            refreshLabel={t("state.retry")}
          />
        ) : null}
        {agentsLoading ? (
          <UiResourceState
            size="sm"
            state="loading"
            title={t("capability.skills_detail_bindings_loading")}
            variant="plain"
          />
        ) : agentBindings.length === 0 && !bindingsFailure ? (
          <UiResourceState
            size="sm"
            state="empty"
            title={t("capability.skills_detail_no_agents")}
            variant="plain"
          />
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
                      <p className={cn(
                        "truncate",
                        getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }),
                      )}>
                        {binding.agent_name}
                      </p>
                      <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
                        {presentation.description}
                      </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      <span className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>
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
    <UiResourceState
      className={className}
      impact={failure.impact}
      primaryAction={onRefresh
        ? { label: refreshLabel, onClick: onRefresh }
        : undefined}
      size="sm"
      state="error"
      title={failure.title}
      tone={failure.tone}
      variant="card"
    />
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
    <CapabilityDetailIdentity
      actions={model.canDelete || model.canUpdate ? (
        <SkillDetailActions
          activeAction={activeAction}
          canDelete={model.canDelete}
          canUpdate={model.canUpdate}
          onDelete={onDelete}
          onUpdate={onUpdate}
        />
      ) : undefined}
      description={model.description}
      leading={<UiSeededAvatar seed={model.avatarSeed} size="lg" />}
      title={<span className="truncate">{model.displayName}</span>}
    />
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
    <>
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
    </>
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
    <UiLinkButton
      href={sourceUrl}
      rel="noopener noreferrer"
      size="sm"
      target="_blank"
      tone="primary"
      variant="text"
    >
      <ExternalLink className="h-3.5 w-3.5" />
      {t("capability.skills_detail_view_source")}
    </UiLinkButton>
  );
}
