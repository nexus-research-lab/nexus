"use client";

import { useEffect } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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
  const accessBlocked = Boolean(memory.resource.error?.access);
  const cancelDeleteDocument = memory.document.cancelDeleteDocument;
  useEffect(() => {
    if (accessBlocked) {
      cancelDeleteDocument();
    }
  }, [accessBlocked, cancelDeleteDocument]);
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
        isOpen={!accessBlocked && deleteTarget !== null}
        message={!accessBlocked && deleteTarget
          ? t("capability.memory_delete_confirm", { name: deleteTarget.title })
          : ""}
        onCancel={cancelDeleteDocument}
        onConfirm={() => {
          if (accessBlocked) {
            cancelDeleteDocument();
            return;
          }
          void memory.document.confirmDeleteDocument();
        }}
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
  if (memory.resource.error?.access) {
    return (
      <UiResourceState
        description={memory.resource.error.message}
        impact={t("state.access_failure_impact")}
        nextStep={t("state.permission_next_step")}
        primaryAction={{
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("state.retry"),
          onClick: () => void memory.resource.refresh(),
        }}
        size="sm"
        state="error"
        title={t("state.permission_title")}
      />
    );
  }
  if (memory.resource.isLoading && !memory.resource.snapshot) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-muted)">
        <LoaderCircle className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (memory.resource.error && !memory.resource.snapshot) {
    return (
      <UiResourceState
        description={memory.resource.error.message}
        impact={t("state.read_failure_impact")}
        nextStep={t("state.retry_next_step")}
        primaryAction={{
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("state.retry"),
          onClick: () => void memory.resource.refresh(),
        }}
        size="sm"
        state="error"
        title={t("capability.memory_load_failed")}
      />
    );
  }
  return (
    <>
      {memory.resource.error ? (
        <UiResourceState
          className="mx-3 mt-3 min-h-0 py-3"
          description={memory.resource.error.message}
          impact={t("capability.memory_stale_catalog_impact")}
          nextStep={t("state.retry_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: () => void memory.resource.refresh(),
          }}
          role="status"
          size="sm"
          state="error"
          title={t("capability.memory_refresh_failed")}
        />
      ) : null}
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
    </>
  );
}
