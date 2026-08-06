import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { AgentOptionsSkillsContent } from "./agent-options-skills-content";
import type { AgentSkillsProjection } from "./agent-skills-model";
import "./agent-options-skills.css";

interface AgentOptionsSkillsViewProps {
  agentId: string | null;
  busySkillName: string | null;
  cancelDisable: () => void;
  commandBusy: boolean;
  confirmDisable: () => void;
  errorMessage: string | null;
  loading: boolean;
  pendingDisableSkill: AgentSkillEntry | null;
  projection: AgentSkillsProjection;
  requestSkillAction: (skill: AgentSkillEntry) => void;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
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
  cancelDisable,
  commandBusy,
  confirmDisable,
  errorMessage,
  loading,
  pendingDisableSkill,
  projection,
  requestSkillAction,
  searchQuery,
  setSearchQuery,
}: AgentOptionsSkillsViewProps) {
  const { t } = useI18n();

  return (
    <div className="agent-options-skills-container space-y-5 animate-in slide-in-from-right-4 duration-300">
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
        confirmText={t("agent_options.skills.disable_confirm_action")}
        isOpen={Boolean(pendingDisableSkill)}
        message={t("agent_options.skills.disable_confirm_message", {
          name: pendingDisableSkill?.title || pendingDisableSkill?.name || "",
        })}
        onCancel={cancelDisable}
        onConfirm={confirmDisable}
        title={t("agent_options.skills.disable_confirm_title")}
        variant="danger"
      />
    </div>
  );
}
