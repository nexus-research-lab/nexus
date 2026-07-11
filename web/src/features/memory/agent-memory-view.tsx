"use client";

import { Brain, LoaderCircle, RefreshCw } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiIconButton } from "@/shared/ui/button";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";

import { AgentMemoryCatalog } from "./agent-memory-catalog";
import { MemoryDocumentPanel } from "./memory-document-panel";
import { formatMemoryModifiedTime } from "./memory-utils";
import { useAgentMemory } from "./use-agent-memory";
import "./memory-view.css";

interface AgentMemoryViewProps {
  agent: Agent;
  agents?: Agent[];
  onAgentChange?: (agentId: string) => void;
}

export function AgentMemoryView({ agent, agents, onAgentChange }: AgentMemoryViewProps) {
  const { locale, t } = useI18n();
  const memory = useAgentMemory(
    agent.agent_id,
    t("capability.memory_load_failed"),
  );

  return (
    <div
      className="nexus-memory-view flex min-h-0 min-w-0 flex-1 flex-col"
      data-document-open={memory.compactDocumentOpen ? "true" : "false"}
    >
      <div className="nexus-memory-summary flex shrink-0 flex-wrap items-center gap-x-5 gap-y-2 border-b border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-raised-background)_52%,transparent)] px-4 py-3">
        {agents && onAgentChange ? (
          <div className="nexus-memory-agent-switcher flex min-w-[210px] max-w-[300px] flex-1 items-center gap-2.5">
            <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />
            <UiSelectMenu
              ariaLabel={t("capability.memory_agent_aria")}
              buttonClassName="rounded-[8px]"
              onChange={onAgentChange}
              options={agents.map((item) => ({ value: item.agent_id, label: item.name }))}
              size="sm"
              value={agent.agent_id}
            />
          </div>
        ) : (
          <div className="flex min-w-0 items-center gap-2">
            <Brain className="h-4 w-4 text-(--icon-muted)" />
            <span className="truncate text-[12px] font-semibold text-(--text-strong)">
              {agent.name}
            </span>
          </div>
        )}

        <div className="nexus-memory-metrics flex min-w-0 flex-1 items-center gap-4 overflow-x-auto text-[11px] text-(--text-soft)">
          <MemoryMetric
            label={t("capability.memory_metric_index")}
            value={memory.snapshot?.index ? t("capability.memory_ready") : "-"}
          />
          <MemoryMetric
            label={t("capability.memory_metric_topics")}
            value={String(memory.counts.topics)}
          />
          <MemoryMetric
            label={t("capability.memory_metric_logs")}
            value={String(memory.counts.logs)}
          />
          <MemoryMetric
            label={t("capability.memory_metric_updated")}
            value={memory.latestDocument
              ? formatMemoryModifiedTime(memory.latestDocument.modified_at, locale)
              : "-"}
          />
        </div>

        <UiIconButton
          aria-label={t("capability.refresh")}
          disabled={memory.isLoading}
          onClick={() => void memory.refresh()}
          size="md"
          title={t("capability.refresh")}
          variant="ghost"
        >
          <RefreshCw className={cn("h-4 w-4", memory.isLoading && "animate-spin")} />
        </UiIconButton>
      </div>

      {memory.isLoading && !memory.snapshot ? (
        <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-muted)">
          <LoaderCircle className="h-5 w-5 animate-spin" />
        </div>
      ) : memory.error ? (
        <UiStateBlock
          description={memory.error}
          size="sm"
          title={t("capability.memory_load_failed")}
        />
      ) : (
        <div className="nexus-memory-layout min-h-0 min-w-0 flex-1">
          <AgentMemoryCatalog
            model={{
              filter: memory.filter,
              indexVisible: memory.indexVisible,
              locale,
              query: memory.query,
              selectedPath: memory.selectedPath,
              snapshot: memory.snapshot,
              visibleDocuments: memory.visibleDocuments,
            }}
            onFilterChange={memory.setFilter}
            onQueryChange={memory.setQuery}
            onSelectDocument={memory.selectDocument}
          />
          <MemoryDocumentPanel
            agentId={agent.agent_id}
            document={memory.selectedDocument}
            onBack={() => memory.setCompactDocumentOpen(false)}
            onSaved={() => void memory.refresh()}
            onSelectPath={memory.selectDocument}
          />
        </div>
      )}
    </div>
  );
}

function MemoryMetric({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex shrink-0 items-baseline gap-1.5">
      <span>{label}</span>
      <strong className="font-semibold tabular-nums text-(--text-default)">{value}</strong>
    </span>
  );
}
