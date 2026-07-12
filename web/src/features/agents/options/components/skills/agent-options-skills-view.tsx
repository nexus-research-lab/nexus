import { RefreshCw } from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { AgentOptionsSkillsContent } from "./agent-options-skills-content";
import type { AgentSkillsProjection } from "./agent-skills-model";

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

function SkillsHeader({
  agentId,
  loading,
  projection,
  refresh,
}: Pick<
  AgentOptionsSkillsViewProps,
  "agentId" | "loading" | "projection" | "refresh"
>) {
  const { t } = useI18n();
  return (
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
  );
}

function SkillsLoadError({
  errorMessage,
}: Pick<AgentOptionsSkillsViewProps, "errorMessage">) {
  return errorMessage ? (
    <UiStateBlock
      description={errorMessage}
      size="sm"
      title="加载失败"
      tone="danger"
      variant="inset"
    />
  ) : null;
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

  return (
    <div className="space-y-3.5 animate-in slide-in-from-right-4 duration-300">
      <SkillsHeader
        agentId={agentId}
        loading={loading}
        projection={projection}
        refresh={refresh}
      />
      <SkillsLoadError errorMessage={errorMessage} />
      <AgentOptionsSkillsContent
        agentId={agentId}
        busySkillName={busySkillName}
        commandBusy={commandBusy}
        loading={loading}
        projection={projection}
        requestSkillAction={requestSkillAction}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
      />

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
