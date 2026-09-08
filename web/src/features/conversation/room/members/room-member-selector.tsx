/**
 * INPUT: Room Agent 目录、成员选择与管理态 participation_paused 草稿。
 * OUTPUT: 互不混淆的加入/移除主动作和逐成员暂停/恢复按钮。
 * POS: Room 管理弹窗中成员身份与持久参与控制的唯一列表视图。
 */
import { Check, Pause, Play, Plus } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiListRow } from "@/shared/ui/list/list-row";

import type { RoomMemberAgentOption } from "./create-room-dialog-types";

interface RoomMemberSelectorProps {
  agents: RoomMemberAgentOption[];
  canManageParticipation: boolean;
  onQueryChange: (query: string) => void;
  onToggleAgent: (agentId: string) => void;
  onToggleParticipation: (agentId: string) => void;
  pausedAgentIds: Set<string>;
  query: string;
  selectedAgentIds: Set<string>;
}

export function RoomMemberSelector({
  agents,
  canManageParticipation,
  onQueryChange,
  onToggleAgent,
  onToggleParticipation,
  pausedAgentIds,
  query,
  selectedAgentIds,
}: RoomMemberSelectorProps) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
      <UiSearchInput
        aria-label={t("room.search_agent_placeholder")}
        controlSize="md"
        onChange={onQueryChange}
        placeholder={t("room.search_agent_placeholder")}
        value={query}
        variant="dialog"
      />
      <p className="dialog-label">
        {t("room.all_agents", { count: agents.length })}
      </p>
      <div className="surface-radius-lg flex h-[min(36vh,360px)] min-h-0 flex-col overflow-hidden border border-(--surface-panel-border) bg-(--surface-panel-background) p-1.5 max-md:h-auto max-md:min-h-[180px] max-md:max-h-[240px]">
        <div
          className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto"
          data-room-member-selection-list="true"
        >
          {agents.map((agent) => (
            <RoomMemberOption
              agent={agent}
              canManageParticipation={canManageParticipation}
              key={agent.agent_id}
              onToggle={onToggleAgent}
              onToggleParticipation={onToggleParticipation}
              participationPaused={pausedAgentIds.has(agent.agent_id)}
              selected={selectedAgentIds.has(agent.agent_id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

function RoomMemberOption({
  agent,
  canManageParticipation,
  onToggle,
  onToggleParticipation,
  participationPaused,
  selected,
}: {
  agent: RoomMemberAgentOption;
  canManageParticipation: boolean;
  onToggle: (agentId: string) => void;
  onToggleParticipation: (agentId: string) => void;
  participationPaused: boolean;
  selected: boolean;
}) {
  const { t } = useI18n();
  const actionLabel = t(
    selected ? "room.agent_select_remove" : "room.agent_select_add",
    { name: agent.name },
  );
  const SelectionIcon = selected ? Check : Plus;
  const participationActionLabel = t(
    participationPaused ? "room.resume_member" : "room.pause_member",
    { name: agent.name },
  );
  const ParticipationIcon = participationPaused ? Play : Pause;
  return (
    <UiListRow
      active={selected}
      aria-label={actionLabel}
      aria-pressed={selected}
      density="dense"
      leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />}
      onClick={() => onToggle(agent.agent_id)}
      right={(
        <span
          className={cn(
            "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs transition-[background-color,color] duration-(--motion-duration-fast)",
            selected
              ? "bg-(--surface-interactive-hover-background) text-(--brand-action)"
              : "text-(--text-soft)",
          )}
        >
          <SelectionIcon className="h-3 w-3" />
        </span>
      )}
      actions={canManageParticipation && selected ? (
        <UiChoiceButton
          active={participationPaused}
          aria-label={participationActionLabel}
          choiceSize="xs"
          onClick={(event) => {
            event.stopPropagation();
            onToggleParticipation(agent.agent_id);
          }}
          title={participationActionLabel}
          tone="neutral"
        >
          <ParticipationIcon className="h-3 w-3" />
          <span>{t(participationPaused ? "room.resume_participation" : "room.pause_participation")}</span>
        </UiChoiceButton>
      ) : null}
      title={agent.name}
      tooltip={actionLabel}
    />
  );
}
