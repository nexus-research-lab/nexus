import { Loader2, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

import { UiIconButton } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { UiSearchInput } from "@/shared/ui/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { AgentSkillEntry } from "@/types/capability/skill";

import type { AgentSkillsProjection } from "./agent-skills-model";
import { AgentSkillCard } from "./agent-skill-card";

interface AgentOptionsSkillsViewProps {
  agentId: string | null;
  busySkillName: string | null;
  cancelRemove: () => void;
  commandBusy: boolean;
  confirmRemove: () => void;
  errorMessage: string | null;
  loading: boolean;
  pendingRemoveSkill: AgentSkillEntry | null;
  projection: AgentSkillsProjection;
  refresh: () => void;
  requestSkillAction: (skill: AgentSkillEntry) => void;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
}

const EMPTY_AVAILABLE_MESSAGE_KEYS: Record<
  Exclude<AgentSkillsProjection["availableEmptyState"], null>,
  TranslationKey
> = {
  catalog_empty: "agent_options.skills.empty_available",
  no_addable: "agent_options.skills.empty_addable",
  no_search_match: "agent_options.skills.empty_search",
};

function SkillsSectionHeader({
  count,
  title,
}: {
  count: ReactNode;
  title: string;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <h4 className="text-sm font-semibold text-(--text-strong)">{title}</h4>
      <span className="text-xs text-(--text-soft)">{count}</span>
    </div>
  );
}

export function AgentOptionsSkillsView({
  agentId,
  busySkillName,
  cancelRemove,
  commandBusy,
  confirmRemove,
  errorMessage,
  loading,
  pendingRemoveSkill,
  projection,
  refresh,
  requestSkillAction,
  searchQuery,
  setSearchQuery,
}: AgentOptionsSkillsViewProps) {
  const { t } = useI18n();
  const availableEmptyMessage = projection.availableEmptyState
    ? t(EMPTY_AVAILABLE_MESSAGE_KEYS[projection.availableEmptyState])
    : null;

  return (
    <div className="space-y-3.5 animate-in slide-in-from-right-4 duration-300">
      <div className="flex items-center justify-between gap-4">
        <h3 className="text-[16px] font-semibold tracking-tight text-(--text-strong)">
          {t("agent_options.skills.summary", {
            count: projection.installed.length,
          })}
        </h3>
        <div className="flex items-center gap-2">
          <span className="text-[12px] text-(--text-soft)">
            {t("agent_options.skills.total", { count: projection.totalCount })}
          </span>
          <UiIconButton
            aria-label={t("capability.refresh")}
            disabled={!agentId || loading}
            onClick={refresh}
            size="sm"
            title={t("capability.refresh")}
            tone="default"
            variant="ghost"
          >
            <RefreshCw className={loading ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
          </UiIconButton>
        </div>
      </div>

      {errorMessage ? (
        <UiStateBlock
          description={errorMessage}
          size="sm"
          title="加载失败"
          tone="danger"
          variant="inset"
        />
      ) : null}
      {loading ? (
        <UiStateBlock
          className="py-10"
          icon={<Loader2 className="h-4 w-4 animate-spin" />}
          size="sm"
          variant="inset"
        />
      ) : null}
      {!agentId ? (
        <UiStateBlock
          description={t("agent_options.skills.create_first")}
          size="sm"
          variant="inset"
        />
      ) : null}

      {agentId && !loading ? (
        <>
          <section className="space-y-2.5">
            <SkillsSectionHeader
              count={projection.installed.length}
              title={t("agent_options.skills.installed")}
            />
            {projection.installed.length === 0 ? (
              <UiStateBlock
                description={t("agent_options.skills.empty_installed")}
                size="sm"
                variant="inset"
              />
            ) : (
              <div className="grid grid-cols-1 gap-1.5">
                {projection.installed.map((skill) => (
                  <AgentSkillCard
                    actionKind="installed"
                    actionLabel={t("agent_options.skills.remove")}
                    busy={busySkillName === skill.name}
                    commandBusy={commandBusy}
                    key={skill.name}
                    onAction={requestSkillAction}
                    skill={skill}
                  />
                ))}
              </div>
            )}
          </section>

          <section className="space-y-2.5">
            <SkillsSectionHeader
              count={`${projection.visibleAddable.length}/${projection.addable.length}`}
              title={t("agent_options.skills.add")}
            />
            <UiSearchInput
              controlSize="md"
              onChange={setSearchQuery}
              placeholder={t("agent_options.skills.search_placeholder")}
              value={searchQuery}
              variant="dialog"
            />
            {availableEmptyMessage ? (
              <UiStateBlock
                description={availableEmptyMessage}
                size="sm"
                variant="inset"
              />
            ) : (
              <div className="grid grid-cols-1 gap-1.5">
                {projection.visibleAddable.map((skill) => (
                  <AgentSkillCard
                    actionKind="add"
                    actionLabel={t("agent_options.skills.add_button")}
                    busy={busySkillName === skill.name}
                    commandBusy={commandBusy}
                    key={skill.name}
                    onAction={requestSkillAction}
                    skill={skill}
                  />
                ))}
              </div>
            )}
          </section>
        </>
      ) : null}

      <ConfirmDialog
        confirmText={t("agent_options.skills.remove_confirm_action")}
        isOpen={Boolean(pendingRemoveSkill)}
        message={t("agent_options.skills.remove_confirm_message", {
          name: pendingRemoveSkill?.title || pendingRemoveSkill?.name || "",
        })}
        onCancel={cancelRemove}
        onConfirm={confirmRemove}
        title={t("agent_options.skills.remove_confirm_title")}
        variant="danger"
      />
    </div>
  );
}
