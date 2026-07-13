"use client";

import { Brain, LoaderCircle, RefreshCw } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiStateBlock } from "@/shared/ui/display/state-block";
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

type AgentMemoryController = ReturnType<typeof useAgentMemory>;

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
      <MemorySummary
        agent={agent}
        agents={agents}
        locale={locale}
        memory={memory}
        onAgentChange={onAgentChange}
      />
      <MemoryContent agentId={agent.agent_id} memory={memory} />
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

function MemoryAgentIdentity({
  agent,
  agents,
  onAgentChange,
}: AgentMemoryViewProps) {
  const { t } = useI18n();
  if (agents && onAgentChange) {
    return (
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
    );
  }
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Brain className="h-4 w-4 text-(--icon-muted)" />
      <span className="truncate text-[12px] font-semibold text-(--text-strong)">
        {agent.name}
      </span>
    </div>
  );
}

function MemoryMetrics({
  locale,
  memory,
}: {
  locale: string;
  memory: AgentMemoryController;
}) {
  const { t } = useI18n();
  const latestModifiedAt = memory.summary.latestDocument?.modified_at;
  return (
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
        value={latestModifiedAt
          ? formatMemoryModifiedTime(latestModifiedAt, locale)
          : "-"}
      />
    </div>
  );
}

function MemorySummary({
  agent,
  agents,
  locale,
  memory,
  onAgentChange,
}: AgentMemoryViewProps & {
  locale: string;
  memory: AgentMemoryController;
}) {
  const { t } = useI18n();
  return (
    <div className="nexus-memory-summary flex shrink-0 flex-wrap items-center gap-x-5 gap-y-2 border-b border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-raised-background)_52%,transparent)] px-4 py-3">
      <MemoryAgentIdentity
        agent={agent}
        agents={agents}
        onAgentChange={onAgentChange}
      />
      <MemoryMetrics locale={locale} memory={memory} />
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
  );
}

function MemoryContent({
  agentId,
  memory,
}: {
  agentId: string;
  memory: AgentMemoryController;
}) {
  const { t } = useI18n();
  if (memory.resource.isLoading && !memory.resource.snapshot) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-muted)">
        <LoaderCircle className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (memory.resource.error) {
    return (
      <UiStateBlock
        description={memory.resource.error}
        size="sm"
        title={t("capability.memory_load_failed")}
      />
    );
  }
  return (
    <div className="nexus-memory-layout min-h-0 min-w-0 flex-1">
      <AgentMemoryCatalog
        emptyFilterVisible={memory.catalog.emptyFilterVisible}
        emptyMemoryVisible={memory.catalog.emptyMemoryVisible}
        filter={memory.catalog.filter}
        onFilterChange={memory.catalog.setFilter}
        onQueryChange={memory.catalog.setQuery}
        onSelectDocument={memory.document.selectDocument}
        query={memory.catalog.query}
        sections={memory.catalog.sections}
        truncated={memory.catalog.truncated}
      />
      <MemoryDocumentPanel
        agentId={agentId}
        document={memory.document.selectedDocument}
        onBack={memory.document.closeCompactDocument}
        onSaved={memory.resource.refresh}
        onSelectPath={memory.document.selectDocument}
      />
    </div>
  );
}
