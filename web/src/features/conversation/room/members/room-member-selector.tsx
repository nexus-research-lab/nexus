import { Check, Plus } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiSearchInput } from "@/shared/ui/form/form-control";

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
      <div className="surface-radius-lg flex max-h-[min(36vh,360px)] min-h-0 flex-col overflow-hidden bg-(--surface-panel-background) p-1.5 max-md:min-h-[180px] max-md:max-h-[240px]">
        <div
          className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto"
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
        "radius-control-md flex min-h-10 w-full cursor-pointer items-center gap-2.5 border border-transparent px-2.5 py-1.5 text-left transition-[background,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
        selected
          ? "bg-(--surface-interactive-active-background)"
          : "bg-transparent hover:bg-(--surface-interactive-hover-background)",
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
          "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs transition-[background-color,color] duration-(--motion-duration-fast)",
          selected
            ? "bg-(--surface-interactive-hover-background) text-(--brand-action)"
            : "text-(--text-soft)",
        )}
      >
        <SelectionIcon className="h-3 w-3" />
      </div>
    </button>
  );
}
