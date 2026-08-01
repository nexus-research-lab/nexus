"use client";

import { LoaderCircle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type { Agent } from "@/types/agent/agent";

import { AgentMemoryCatalog } from "./catalog/agent-memory-catalog";
import { useAgentMemory } from "./catalog/use-agent-memory";
import { MemoryDocumentPanel } from "./document/memory-document-panel";
import "./memory-view.css";

interface AgentMemoryViewProps {
  agent: Agent;
}

type AgentMemoryController = ReturnType<typeof useAgentMemory>;

export function AgentMemoryView({ agent }: AgentMemoryViewProps) {
  const { t } = useI18n();
  const memory = useAgentMemory(
    agent.agent_id,
    t("capability.memory_load_failed"),
    t("capability.memory_delete_failed"),
  );
  const deleteTarget = memory.document.deleteTarget;
  return (
    <>
      <div
        className="nexus-memory-view flex min-h-0 min-w-0 flex-1 flex-col"
        data-document-open={memory.document.compactDocumentOpen ? "true" : "false"}
      >
        <MemoryContent agentId={agent.agent_id} memory={memory} />
      </div>
      <ConfirmDialog
        confirmText={t("capability.memory_delete")}
        isOpen={deleteTarget !== null}
        message={deleteTarget
          ? t("capability.memory_delete_confirm", { name: deleteTarget.title })
          : ""}
        onCancel={memory.document.cancelDeleteDocument}
        onConfirm={() => void memory.document.confirmDeleteDocument()}
        title={t("capability.memory_delete")}
        variant="danger"
      />
    </>
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
        onRefresh={() => void memory.resource.refresh()}
        onSelectDocument={memory.document.selectDocument}
        query={memory.catalog.query}
        refreshing={memory.resource.isLoading}
        sections={memory.catalog.sections}
        truncated={memory.catalog.truncated}
      />
      <MemoryDocumentPanel
        agentId={agentId}
        deleteBusy={Boolean(memory.document.deletingPath)}
        deleteError={memory.document.deleteError}
        deleting={memory.document.deletingPath === memory.document.selectedDocument?.path}
        document={memory.document.selectedDocument}
        onBack={memory.document.closeCompactDocument}
        onDelete={() => {
          const selectedPath = memory.document.selectedDocument?.path;
          if (selectedPath) {
            memory.document.requestDeleteDocument(selectedPath);
          }
        }}
        onSaved={memory.resource.refresh}
        onSelectPath={memory.document.selectDocument}
      />
    </div>
  );
}
