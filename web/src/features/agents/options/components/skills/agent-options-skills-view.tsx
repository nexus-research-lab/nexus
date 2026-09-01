// INPUT: Agent Skill 读取/修改可靠性状态与列表交互。
// OUTPUT: 分离的三问错误面、同意图解锁动作和 Skill 列表。
// POS: Agent Options Skill 展示边界；读取刷新不清理 mutation unknown。
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { AgentOptionsSkillsContent } from "./agent-options-skills-content";
import type {
  AgentSkillMutationFailure,
  AgentSkillsProjection,
  AgentSkillsReadFailure,
} from "./agent-skills-model";
import "./agent-options-skills.css";

interface AgentOptionsSkillsViewProps {
  agentId: string | null;
  busySkillName: string | null;
  cancelDisable: () => void;
  commandBusy: boolean;
  confirmDisable: () => void;
  loading: boolean;
  mutationFailures: AgentSkillMutationFailure[];
  pendingDisableSkill: AgentSkillEntry | null;
  projection: AgentSkillsProjection;
  readFailure: AgentSkillsReadFailure | null;
  refresh: () => Promise<unknown>;
  requestSkillAction: (skill: AgentSkillEntry) => void;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  blockedSkillNames: ReadonlySet<string>;
}

function SkillsLoadError({
  failure,
  loading,
  refresh,
}: {
  failure: AgentSkillsReadFailure | null;
  loading: boolean;
  refresh: () => Promise<unknown>;
}) {
  const { t } = useI18n();
  return failure ? (
    <UiResourceState
      impact={failure.impact}
      primaryAction={{
        busy: loading,
        label: t("state.retry"),
        onClick: () => void refresh(),
      }}
      size="sm"
      state="error"
      title={failure.title}
      variant="inset"
    />
  ) : null;
}

function SkillMutationFailures({
  failures,
  refresh,
}: {
  failures: AgentSkillMutationFailure[];
  refresh: () => Promise<unknown>;
}) {
  const { t } = useI18n();
  const failure = failures[0];
  return failure ? (
    <UiResourceState
      impact={failure.impact}
      primaryAction={failure.blocksRepeat ? {
        label: t("state.reload_check"),
        onClick: () => void refresh(),
      } : undefined}
      size="sm"
      state="error"
      title={failure.title}
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
  blockedSkillNames,
  loading,
  mutationFailures,
  pendingDisableSkill,
  projection,
  readFailure,
  refresh,
  requestSkillAction,
  searchQuery,
  setSearchQuery,
}: AgentOptionsSkillsViewProps) {
  const { t } = useI18n();

  return (
    <div className="agent-options-skills-container space-y-5 animate-in slide-in-from-right-4 duration-300">
      <SkillsLoadError
        failure={readFailure}
        loading={loading}
        refresh={refresh}
      />
      <SkillMutationFailures
        failures={mutationFailures}
        refresh={refresh}
      />
      <AgentOptionsSkillsContent
        agentId={agentId}
        blockedSkillNames={blockedSkillNames}
        busySkillName={busySkillName}
        commandBusy={commandBusy}
        loading={loading}
        readBlocked={Boolean(readFailure)}
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
