import { Loader2 } from "lucide-react";
import type { ReactNode } from "react";

import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { AgentSkillEntry } from "@/types/capability/skill";

import { AgentSkillCard } from "./agent-skill-card";
import type { AgentSkillsProjection } from "./agent-skills-model";

export interface AgentOptionsSkillsContentProps {
  agentId: string | null;
  busySkillName: string | null;
  commandBusy: boolean;
  loading: boolean;
  projection: AgentSkillsProjection;
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

function InstalledSkillsSection({
  busySkillName,
  commandBusy,
  projection,
  requestSkillAction,
}: Pick<
  AgentOptionsSkillsContentProps,
  "busySkillName" | "commandBusy" | "projection" | "requestSkillAction"
>) {
  const { t } = useI18n();
  return (
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
  );
}

function AvailableSkillsSection({
  busySkillName,
  commandBusy,
  projection,
  requestSkillAction,
  searchQuery,
  setSearchQuery,
}: Pick<
  AgentOptionsSkillsContentProps,
  | "busySkillName"
  | "commandBusy"
  | "projection"
  | "requestSkillAction"
  | "searchQuery"
  | "setSearchQuery"
>) {
  const { t } = useI18n();
  const emptyMessage = projection.availableEmptyState
    ? t(EMPTY_AVAILABLE_MESSAGE_KEYS[projection.availableEmptyState])
    : null;
  return (
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
      {emptyMessage ? (
        <UiStateBlock
          description={emptyMessage}
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
        variant="inset"
      />
    );
  }
  if (props.loading) {
    return (
      <UiStateBlock
        className="py-10"
        icon={<Loader2 className="h-4 w-4 animate-spin" />}
        size="sm"
        variant="inset"
      />
    );
  }
  return (
    <>
      <InstalledSkillsSection {...props} />
      <AvailableSkillsSection {...props} />
    </>
  );
}
