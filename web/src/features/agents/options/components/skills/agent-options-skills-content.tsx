// INPUT: 已投影 Skill 列表、读取可用性与逐 Skill mutation 锁。
// OUTPUT: 保留快照且仅禁用不安全开关的分组列表。
// POS: Agent Options Skill 内容层；不解释失败、不触发自动重试。
import { Loader2 } from "lucide-react";
import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { AgentSkillCard } from "./agent-skill-card";
import type { AgentSkillsProjection } from "./agent-skills-model";

export interface AgentOptionsSkillsContentProps {
  agentId: string | null;
  blockedSkillNames: ReadonlySet<string>;
  busySkillName: string | null;
  commandBusy: boolean;
  loading: boolean;
  projection: AgentSkillsProjection;
  requestSkillAction: (skill: AgentSkillEntry) => void;
  readBlocked: boolean;
  searchQuery: string;
  setSearchQuery: (value: string) => void;
}

const EMPTY_AVAILABLE_MESSAGE_KEYS: Record<
  Exclude<AgentSkillsProjection["availableEmptyState"], null>,
  TranslationKey
> = {
  catalog_empty: "agent_options.skills.empty_available",
  no_available: "agent_options.skills.empty_available_more",
  no_search_match: "agent_options.skills.empty_search",
};

const AGENT_SKILL_GRID_CLASS_NAME = "agent-options-skills-grid";

function SkillsSectionHeader({
  title,
  trailing,
}: {
  title: string;
  trailing?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <h4 className="text-sm font-semibold text-(--text-strong)">{title}</h4>
      {trailing}
    </div>
  );
}

function EnabledSkillsSection({
  busySkillName,
  blockedSkillNames,
  commandBusy,
  projection,
  requestSkillAction,
  readBlocked,
}: Pick<
  AgentOptionsSkillsContentProps,
  | "blockedSkillNames"
  | "busySkillName"
  | "commandBusy"
  | "projection"
  | "readBlocked"
  | "requestSkillAction"
>) {
  const { t } = useI18n();
  return (
    <section className="space-y-3.5">
      <SkillsSectionHeader
        title={t("agent_options.skills.enabled_section")}
        trailing={(
          <span className="text-xs text-(--text-soft)">
            {projection.enabled.length}
          </span>
        )}
      />
      {projection.enabled.length === 0 ? (
        <UiStateBlock
          description={t("agent_options.skills.empty_enabled")}
          size="sm"
          variant="card"
        />
      ) : (
        <div className={AGENT_SKILL_GRID_CLASS_NAME}>
          {projection.enabled.map((skill) => (
            <AgentSkillCard
              actionLabel={t("agent_options.skills.disable")}
              busy={busySkillName === skill.name}
              blocked={blockedSkillNames.has(skill.name) || readBlocked}
              commandBusy={commandBusy}
              key={skill.name}
              onAction={requestSkillAction}
              skill={skill}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function AvailableSkillsSection({
  busySkillName,
  blockedSkillNames,
  commandBusy,
  projection,
  requestSkillAction,
  readBlocked,
  searchQuery,
  setSearchQuery,
}: Pick<
  AgentOptionsSkillsContentProps,
  | "busySkillName"
  | "blockedSkillNames"
  | "commandBusy"
  | "projection"
  | "readBlocked"
  | "requestSkillAction"
  | "searchQuery"
  | "setSearchQuery"
>) {
  const { t } = useI18n();
  const emptyMessage = projection.availableEmptyState
    ? t(EMPTY_AVAILABLE_MESSAGE_KEYS[projection.availableEmptyState])
    : null;
  const filteredCount = searchQuery.trim()
    ? `${projection.visibleAvailable.length}/${projection.available.length}`
    : null;
  return (
    <section className="space-y-3.5">
      <SkillsSectionHeader
        title={t("agent_options.skills.available_section")}
        trailing={(
          <div className="flex w-full items-center gap-3 sm:w-auto">
            {filteredCount ? (
              <span className="shrink-0 text-xs text-(--text-soft)">
                {filteredCount}
              </span>
            ) : null}
            <UiSearchInput
              className="min-w-0 flex-1 sm:w-[288px] sm:flex-none"
              controlSize="md"
              onChange={setSearchQuery}
              placeholder={t("agent_options.skills.search_placeholder")}
              value={searchQuery}
              variant="dialog"
            />
          </div>
        )}
      />
      {emptyMessage ? (
        <UiStateBlock
          description={emptyMessage}
          size="sm"
          variant="card"
        />
      ) : (
        <div className={AGENT_SKILL_GRID_CLASS_NAME}>
          {projection.visibleAvailable.map((skill) => (
            <AgentSkillCard
              actionLabel={t("agent_options.skills.enable")}
              busy={busySkillName === skill.name}
              blocked={blockedSkillNames.has(skill.name) || readBlocked}
              commandBusy={commandBusy}
              key={skill.name}
              onAction={requestSkillAction}
              skill={skill}
            />
          ))}
        </div>
      )}
    </section>
  );
}

export function AgentOptionsSkillsContent(
  props: AgentOptionsSkillsContentProps,
) {
  const { t } = useI18n();
  if (!props.agentId) {
    return (
      <UiStateBlock
        description={t("agent_options.skills.create_first")}
        size="sm"
        variant="card"
      />
    );
  }
  if (props.loading) {
    return (
      <UiStateBlock
        className="py-10"
        icon={(
          <Loader2
            className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
          />
        )}
        size="sm"
        variant="card"
      />
    );
  }
  return (
    <>
      <EnabledSkillsSection {...props} />
      <AvailableSkillsSection {...props} />
    </>
  );
}
