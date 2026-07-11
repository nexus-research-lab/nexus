"use client";

import { Brain, LoaderCircle, RefreshCw } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiIconButton } from "@/shared/ui/button";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";

import { AgentMemoryCatalog } from "./catalog/agent-memory-catalog";
import { useAgentMemory } from "./catalog/use-agent-memory";
import { MemoryDocumentPanel } from "./document/memory-document-panel";
import { formatMemoryModifiedTime } from "./memory-utils";
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
      data-document-open={memory.document.compactDocumentOpen ? "true" : "false"}
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
            value={memory.resource.snapshot?.index ? t("capability.memory_ready") : "-"}
          />
          <MemoryMetric
            label={t("capability.memory_metric_topics")}
            value={String(memory.summary.counts.topics)}
          />
          <MemoryMetric
            label={t("capability.memory_metric_logs")}
            value={String(memory.summary.counts.logs)}
          />
          <MemoryMetric
            label={t("capability.memory_metric_updated")}
            value={memory.summary.latestDocument
              ? formatMemoryModifiedTime(memory.summary.latestDocument.modified_at, locale)
              : "-"}
          />
        </div>

        <UiIconButton
          aria-label={t("capability.refresh")}
          disabled={memory.resource.isLoading}
          onClick={() => void memory.resource.refresh()}
          size="md"
          title={t("capability.refresh")}
          variant="ghost"
        >
          <RefreshCw className={cn(
            "h-4 w-4",
            memory.resource.isLoading && "animate-spin",
          )} />
        </UiIconButton>
      </div>

      {memory.resource.isLoading && !memory.resource.snapshot ? (
        <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-muted)">
          <LoaderCircle className="h-5 w-5 animate-spin" />
        </div>
      ) : memory.resource.error ? (
        <UiStateBlock
          description={memory.resource.error}
          size="sm"
          title={t("capability.memory_load_failed")}
        />
      ) : (
        <div className="nexus-memory-layout min-h-0 min-w-0 flex-1">
          <AgentMemoryCatalog
            filter={memory.catalog.filter}
            indexVisible={memory.catalog.indexVisible}
            onFilterChange={memory.catalog.setFilter}
            onQueryChange={memory.catalog.setQuery}
            onSelectDocument={memory.document.selectDocument}
            query={memory.catalog.query}
            selectedPath={memory.catalog.selectedPath}
            snapshot={memory.catalog.snapshot}
            visibleDocuments={memory.catalog.visibleDocuments}
          />
          <MemoryDocumentPanel
            agentId={agent.agent_id}
            document={memory.document.selectedDocument}
            onBack={memory.document.closeCompactDocument}
            onSaved={memory.resource.refresh}
            onSelectPath={memory.document.selectDocument}
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
