"use client";

import { ChevronDown, Grid2X2, Plus } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn, truncate } from "@/lib/utils";

interface AgentSwitcherProps {
  agents: Agent[];
  currentAgentId: string | null;
  recentAgents: Agent[];
  onSelectAgent: (agentId: string) => void;
  onOpenDirectory: () => void;
  onCreateAgent: () => void;
}

export function AgentSwitcher({
  agents,
  currentAgentId,
  recentAgents,
  onSelectAgent,
  onOpenDirectory,
  onCreateAgent,
}: AgentSwitcherProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex items-center gap-2 rounded-full border border-border/80 bg-secondary px-3 py-2 shadow-sm">
        <Grid2X2 className="h-4 w-4 text-muted-foreground" />
        <button
          onClick={onOpenDirectory}
          className="text-sm font-medium text-foreground transition-colors hover:text-primary"
          type="button"
        >
          Agents
        </button>
      </div>

      <div className="flex items-center gap-2 rounded-full border border-border/80 bg-card px-3 py-2 shadow-sm">
        <div className="relative">
          <select
            aria-label="选择 Agent"
            className="appearance-none bg-transparent pr-6 text-sm font-semibold text-foreground outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            onChange={(event) => onSelectAgent(event.target.value)}
            value={currentAgentId ?? ""}
          >
            <option disabled value="">
              选择一个 Agent
            </option>
            {agents.map((agent) => (
              <option key={agent.agent_id} value={agent.agent_id}>
                {agent.name}
              </option>
            ))}
          </select>
          <ChevronDown className="pointer-events-none absolute right-0 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {recentAgents.map((agent) => {
          const isActive = agent.agent_id === currentAgentId;
          return (
            <button
              key={agent.agent_id}
              className={cn(
                "rounded-full border px-3 py-2 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-primary/50",
                isActive
                  ? "border-primary/30 bg-primary/10 text-primary"
                  : "border-border/80 bg-secondary text-muted-foreground hover:border-primary/20 hover:text-foreground",
              )}
              onClick={() => onSelectAgent(agent.agent_id)}
              type="button"
            >
              {truncate(agent.name, 14)}
            </button>
          );
        })}
      </div>

      <button
        className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-secondary px-3 py-2 text-sm font-medium text-foreground shadow-sm transition-colors hover:border-primary/20 hover:text-primary focus-visible:ring-2 focus-visible:ring-primary/50"
        onClick={onCreateAgent}
        type="button"
      >
        <Plus className="h-4 w-4" />
        新建 Agent
      </button>
    </div>
  );
}
