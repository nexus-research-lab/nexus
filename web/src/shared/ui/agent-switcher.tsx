"use client";

import { ChevronDown, Grid2X2, Plus } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn, truncate } from "@/lib/utils";

interface AgentSwitcherProps {
  agents: Agent[];
  current_agent_id: string | null;
  recent_agents: Agent[];
  on_select_agent: (agent_id: string) => void;
  on_open_directory: () => void;
  on_create_agent: () => void;
}

export function AgentSwitcher({
  agents,
  current_agent_id,
  recent_agents,
  on_select_agent,
  on_open_directory,
  on_create_agent,
}: AgentSwitcherProps) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="workspace-chip flex items-center gap-2 rounded-full px-4 py-2.5">
        <Grid2X2 className="h-4 w-4 text-slate-700/54" />
        <button
          onClick={on_open_directory}
          className="text-sm font-semibold text-slate-900/84 transition-colors hover:text-slate-950"
          type="button"
        >
          Agents
        </button>
      </div>

      <div className="workspace-chip flex items-center gap-2 rounded-full px-4 py-2.5">
        <div className="relative">
          <select
            aria-label="选择 Agent"
            className="appearance-none bg-transparent pr-6 text-sm font-semibold text-slate-900/84 outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
            onChange={(event) => on_select_agent(event.target.value)}
            value={current_agent_id ?? ""}
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
          <ChevronDown className="pointer-events-none absolute right-0 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-700/50" />
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {recent_agents.map((agent) => {
          const isActive = agent.agent_id === current_agent_id;
          return (
            <button
              key={agent.agent_id}
              className={cn(
                "rounded-full px-3.5 py-2 text-sm transition-all duration-300 focus-visible:ring-2 focus-visible:ring-primary/40",
                isActive
                  ? "workspace-card-strong text-slate-950 shadow-[0_14px_26px_rgba(111,126,162,0.12)]"
                  : "workspace-chip text-slate-700/72 hover:-translate-y-0.5 hover:text-slate-950",
              )}
              onClick={() => on_select_agent(agent.agent_id)}
              type="button"
            >
              {truncate(agent.name, 14)}
            </button>
          );
        })}
      </div>

      <button
        className="workspace-chip inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm font-semibold text-slate-900/84 transition-all duration-300 hover:-translate-y-0.5 hover:text-slate-950 focus-visible:ring-2 focus-visible:ring-primary/40"
        onClick={on_create_agent}
        type="button"
      >
        <Plus className="h-4 w-4" />
        新建 Agent
      </button>
    </div>
  );
}
