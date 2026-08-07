/**
 * INPUT: Room Agent 目录、成员选择与管理态 participation_paused 草稿。
 * OUTPUT: 互不混淆的加入/移除主动作和逐成员暂停/恢复按钮。
 * POS: Room 管理弹窗中成员身份与持久参与控制的唯一列表视图。
 */
import { Check, Pause, Play, Plus } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiSearchInput } from "@/shared/ui/form/form-control";

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
    <div
      className={cn(
        "radius-control-md flex min-h-10 w-full items-center border border-transparent transition-[background,color] duration-(--motion-duration-fast)",
        selected
          ? "bg-(--surface-interactive-active-background)"
          : "bg-transparent hover:bg-(--surface-interactive-hover-background)",
      )}
    >
      <button
        aria-label={actionLabel}
        aria-pressed={selected}
        className="flex min-w-0 flex-1 cursor-pointer items-center gap-2.5 px-2.5 py-1.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]"
        onClick={() => onToggle(agent.agent_id)}
        title={actionLabel}
        type="button"
      >
        <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />
        <p className="min-w-0 flex-1 truncate text-sm font-semibold text-(--text-strong)">
          {agent.name}
        </p>
        <div
          className={cn(
            "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs transition-[background-color,color] duration-(--motion-duration-fast)",
            selected
              ? "bg-(--surface-interactive-hover-background) text-(--brand-action)"
              : "text-(--text-soft)",
          )}
        >
          <SelectionIcon className="h-3 w-3" />
        </div>
      </button>
      {canManageParticipation && selected ? (
        <button
          aria-label={participationActionLabel}
          aria-pressed={participationPaused}
          className={cn(
            "mr-1.5 flex h-7 shrink-0 items-center gap-1 radius-control-xs px-2 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
            participationPaused
              ? "bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning) hover:bg-[color:color-mix(in_srgb,var(--warning)_16%,transparent)]"
              : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
          )}
          onClick={() => onToggleParticipation(agent.agent_id)}
          title={participationActionLabel}
          type="button"
        >
          <ParticipationIcon className="h-3 w-3" />
          <span>{t(participationPaused ? "room.resume_participation" : "room.pause_participation")}</span>
        </button>
      ) : null}
    </div>
  );
}
