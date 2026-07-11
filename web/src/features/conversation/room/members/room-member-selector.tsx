import { Check, Plus, Search } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";

import type { RoomMemberAgentOption } from "./create-room-dialog-types";

interface RoomMemberSelectorProps {
  agents: RoomMemberAgentOption[];
  onQueryChange: (query: string) => void;
  onToggleAgent: (agentId: string) => void;
  query: string;
  selectedAgentIds: Set<string>;
}

export function RoomMemberSelector({
  agents,
  onQueryChange,
  onToggleAgent,
  query,
  selectedAgentIds,
}: RoomMemberSelectorProps) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-(--text-soft)" />
        <input
          aria-label={t("room.search_agent_placeholder")}
          className="dialog-input w-full rounded-xl py-2 pl-8 pr-3 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={t("room.search_agent_placeholder")}
          type="text"
          value={query}
        />
      </div>
      <p className="dialog-label">
        {t("room.all_agents", { count: agents.length })}
      </p>
      <div className="flex max-h-[min(36vh,360px)] min-h-0 flex-col overflow-hidden rounded-[16px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_84%,transparent)] px-2 py-2">
        <div
          className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto pr-1"
          data-room-member-selection-list="true"
        >
          {agents.map((agent) => (
            <RoomMemberOption
              agent={agent}
              key={agent.agent_id}
              onToggle={onToggleAgent}
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
  onToggle,
  selected,
}: {
  agent: RoomMemberAgentOption;
  onToggle: (agentId: string) => void;
  selected: boolean;
}) {
  const { t } = useI18n();
  const actionLabel = t(
    selected ? "room.agent_select_remove" : "room.agent_select_add",
    { name: agent.name },
  );
  const SelectionIcon = selected ? Check : Plus;
  return (
    <button
      aria-label={actionLabel}
      aria-pressed={selected}
      className={cn(
        "flex w-full cursor-pointer items-center gap-3 rounded-[14px] border px-3 py-1.5 text-left transition-[background,border-color] duration-(--motion-duration-normal)",
        selected
          ? "border-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)]"
          : "border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] bg-transparent hover:border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)]",
      )}
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
          "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center rounded-full transition-all",
          selected
            ? "bg-primary text-white"
            : "border border-(--surface-interactive-hover-border) text-(--text-soft)",
        )}
      >
        <SelectionIcon className="h-3 w-3" />
      </div>
    </button>
  );
}
