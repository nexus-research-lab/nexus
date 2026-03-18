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
      <div className="neo-pill flex items-center gap-2 rounded-full px-4 py-2.5">
        <Grid2X2 className="h-4 w-4 text-muted-foreground" />
        <button
          onClick={onOpenDirectory}
          className="text-sm font-semibold text-foreground transition-colors hover:text-primary"
          type="button"
        >
          Agents
        </button>
      </div>

      <div className="neo-pill flex items-center gap-2 rounded-full px-4 py-2.5">
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
                "rounded-full px-3.5 py-2 text-sm transition-all duration-300 focus-visible:ring-2 focus-visible:ring-primary/50",
                isActive
                  ? "bg-[linear-gradient(135deg,rgba(174,163,255,0.34),rgba(240,237,232,0.92))] text-primary neo-soft-shadow"
                  : "neo-pill text-muted-foreground hover:-translate-y-0.5 hover:text-foreground",
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
        className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm font-semibold text-foreground transition-all duration-300 hover:-translate-y-0.5 hover:text-primary focus-visible:ring-2 focus-visible:ring-primary/50"
        onClick={onCreateAgent}
        type="button"
      >
        <Plus className="h-4 w-4" />
        新建 Agent
      </button>
    </div>
  );
}
