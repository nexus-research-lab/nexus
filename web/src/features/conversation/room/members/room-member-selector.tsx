/**
 * INPUT: Room Agent/Team 真人目录、成员选择与管理态 participation_paused 草稿。
 * OUTPUT: 统一成员列表中的邀请/加入动作和逐 Agent 暂停/恢复按钮。
 * POS: Room 创建与管理弹窗的成员选择视图。
 */
import { Check, Pause, Play, Plus } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiListRow } from "@/shared/ui/list/list-row";

import type { RoomMemberAgentOption } from "./create-room-dialog-types";
import type { RoomMemberUserOption } from "./create-room-dialog-types";

interface RoomMemberSelectorProps {
  agents: RoomMemberAgentOption[];
  canManageParticipation: boolean;
  disabled?: boolean;
  onQueryChange: (query: string) => void;
  onToggleAgent: (agentId: string) => void;
  onToggleParticipation: (agentId: string) => void;
  onToggleUser: (userId: string) => void;
  pausedAgentIds: Set<string>;
  query: string;
  selectedAgentIds: Set<string>;
  selectedUserIds: Set<string>;
  users: RoomMemberUserOption[];
}

export function RoomMemberSelector({
  agents,
  canManageParticipation,
  disabled = false,
  onQueryChange,
  onToggleAgent,
  onToggleParticipation,
  onToggleUser,
  pausedAgentIds,
  query,
  selectedAgentIds,
  selectedUserIds,
  users = [],
}: RoomMemberSelectorProps) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
      <UiSearchInput
        aria-label={t(users.length > 0 ? "room.search_member_placeholder" : "room.search_agent_placeholder")}
        controlSize="md"
        disabled={disabled}
        onChange={onQueryChange}
        placeholder={t(users.length > 0 ? "room.search_member_placeholder" : "room.search_agent_placeholder")}
        value={query}
        variant="dialog"
      />
      <p className="dialog-label">
        {t("room.all_members", { count: agents.length + users.length })}
      </p>
      <div className="surface-radius-lg flex h-[min(36vh,360px)] min-h-0 flex-col overflow-hidden border border-(--surface-panel-border) bg-(--surface-panel-background) p-1.5 max-md:h-auto max-md:min-h-[180px] max-md:max-h-[240px]">
        <div
          className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto"
          data-room-member-selection-list="true"
        >
          {users.map((user) => (
            <RoomUserOption
              disabled={disabled}
              key={user.user_id}
              onToggle={onToggleUser}
              selected={selectedUserIds.has(user.user_id)}
              user={user}
            />
          ))}
          {agents.map((agent) => (
            <RoomMemberOption
              agent={agent}
              canManageParticipation={canManageParticipation}
              disabled={disabled}
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

function RoomUserOption({
  disabled,
  onToggle,
  selected,
  user,
}: {
  disabled: boolean;
  onToggle: (userId: string) => void;
  selected: boolean;
  user: RoomMemberUserOption;
}) {
  const { t } = useI18n();
  const name = user.display_name || user.username;
  const actionLabel = t(selected ? "room.user_select_remove" : "room.user_select_add", { name });
  const SelectionIcon = selected ? Check : Plus;
  return (
    <UiListRow
      active={selected}
      aria-label={actionLabel}
      aria-pressed={selected}
      density="dense"
      disabled={disabled}
      leading={<UiAgentAvatar avatar={user.avatar} name={name} size="sm" />}
      onClick={() => onToggle(user.user_id)}
      right={<SelectionIcon aria-hidden="true" className="h-3 w-3" />}
      title={name}
      tooltip={actionLabel}
    />
  );
}

function RoomMemberOption({
  agent,
  canManageParticipation,
  disabled = false,
  onToggle,
  onToggleParticipation,
  participationPaused,
  selected,
}: {
  agent: RoomMemberAgentOption;
  canManageParticipation: boolean;
  disabled?: boolean;
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
      disabled={disabled}
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
          disabled={disabled}
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
