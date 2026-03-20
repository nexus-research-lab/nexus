"use client";

import { ArrowLeft } from "lucide-react";

import { AgentSwitcher } from "@/components/workspace/agent-switcher";
import { Agent } from "@/types/agent";

interface WorkspaceTopBarProps {
  currentAgentName: string;
  agents: Agent[];
  currentAgentId: string | null;
  recentAgents: Agent[];
  onSelectAgent: (agentId: string) => void;
  onOpenDirectory: () => void;
  onCreateAgent: () => void;
}

export function WorkspaceTopBar({
  currentAgentName,
  agents,
  currentAgentId,
  recentAgents,
  onSelectAgent,
  onOpenDirectory,
  onCreateAgent,
}: WorkspaceTopBarProps) {
  return (
    <div className="soft-ring radius-shell-lg panel-surface px-4 py-3">
      <div className="flex items-center justify-between gap-3 md:hidden">
        <button
          className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold text-foreground transition-transform duration-300 hover:-translate-y-0.5 hover:text-primary"
          onClick={onOpenDirectory}
          type="button"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          返回
        </button>

        <div className="min-w-0 flex-1 text-right">
          <p className="truncate text-sm font-semibold text-foreground">{currentAgentName}</p>
        </div>
      </div>

      <div className="hidden flex-wrap items-center justify-between gap-4 md:flex">
        <AgentSwitcher
          agents={agents}
          currentAgentId={currentAgentId}
          recentAgents={recentAgents}
          onSelectAgent={onSelectAgent}
          onOpenDirectory={onOpenDirectory}
          onCreateAgent={onCreateAgent}
        />

        <div className="flex flex-wrap items-center gap-2">
          <button
            className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold text-foreground transition-transform duration-300 hover:-translate-y-0.5 hover:text-primary"
            onClick={onOpenDirectory}
            type="button"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            返回目录
          </button>
        </div>
      </div>
    </div>
  );
}
