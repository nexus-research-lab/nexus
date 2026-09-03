/**
 * INPUT: assistant 消息中持久化的完整 WorkGraph Draft/命名图快照。
 * OUTPUT: 对话内紧凑草图卡片，以及按需加载 exact 来源图并完整解释读取失败的扁平对照弹窗。
 * POS: 普通 DM/Room WorkGraph authoring 最终回复视图；来源读取失败不改变草图、命名图或源 Execution。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CheckCircle2,
  GitCompareArrows,
  GitFork,
  LoaderCircle,
  RotateCcw,
} from "lucide-react";

import { ExecutionWorkGraphCanvas } from "@/features/conversation/shared/execution/execution-workgraph-canvas";
import { NamedWorkGraphSketch } from "@/features/conversation/shared/execution/named-workgraph-sketch";
import { projectWorkGraphWorkflowCanvasExecution } from "@/features/conversation/shared/execution/workgraph-workflow-canvas-model";
import { getExecutionApi } from "@/lib/api/conversation/execution-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphArtifactContent } from "@/types/conversation/message/content";
import type {
  WorkGraphWorkflow,
  WorkGraphWorkflowPreview,
} from "@/types/conversation/workgraph-workflow";

type ComparePane = "source" | "draft";

export function WorkGraphArtifactBlock({
  artifact,
}: {
  artifact: WorkGraphArtifactContent;
}) {
  const { t } = useI18n();
  const [compareOpen, setCompareOpen] = useState(false);
  const graph = artifact.preview ?? artifact.workflow;
  if (!graph) return null;
  const selectedRevision = artifact.selected_revision
    ?? ("version" in graph ? graph.version : 1);
  const stateLabel = artifact.state === "saved"
    ? t("execution.workflow_artifact_saved")
    : t("execution.workflow_artifact_draft");

  return (
    <>
      <article
        className="w-full max-w-3xl overflow-hidden rounded-[14px] border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] bg-(--surface-panel-background) shadow-[0_10px_28px_color-mix(in_srgb,var(--text-strong)_5%,transparent)]"
        data-workgraph-artifact={artifact.state}
      >
        <header className="flex items-start justify-between gap-4 border-b border-(--divider-subtle-color) px-4 py-3">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-(--text-strong)">
                {artifact.state === "saved"
                  ? <CheckCircle2 className="h-3.5 w-3.5 text-(--success)" />
                  : <GitFork className="h-3.5 w-3.5 text-(--primary)" />}
                {stateLabel}
              </span>
              <code className="rounded-md bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] px-1.5 py-0.5 text-[11px] text-(--primary)">
                /{graph.slash_name}
              </code>
              <span className="text-[10px] text-(--text-soft)">v{selectedRevision}</span>
            </div>
            <h3 className="mt-1.5 truncate text-sm font-semibold text-(--text-strong)">{graph.title}</h3>
            {graph.description ? (
              <p className="mt-1 line-clamp-2 text-xs leading-5 text-(--text-muted)">{graph.description}</p>
            ) : null}
          </div>
          <button
            className={getDialogActionClassName("default", "compact")}
            type="button"
            onClick={() => setCompareOpen(true)}
          >
            <GitCompareArrows className="h-3.5 w-3.5" />
            {t("execution.workflow_artifact_compare")}
          </button>
        </header>
        <div className="p-3">
          <NamedWorkGraphSketch
            className="max-h-64"
            dependencies={graph.dependencies}
            nodes={graph.nodes}
          />
          <div className="mt-2.5 flex flex-wrap items-center justify-between gap-2 px-1 text-[10px] text-(--text-soft)">
            <span>{graph.nodes.length} {t("execution.workflow_nodes_short")} · {(graph.dependencies ?? []).length} {t("execution.workflow_artifact_dependencies")}</span>
            {artifact.version_count && artifact.version_count > 1 ? (
              <span>{artifact.version_count} {t("execution.workflow_artifact_versions")}</span>
            ) : null}
          </div>
        </div>
      </article>
      {compareOpen ? (
        <WorkGraphCompareDialog
          artifact={artifact}
          onClose={() => setCompareOpen(false)}
        />
      ) : null}
    </>
  );
}

function WorkGraphCompareDialog({
  artifact,
  onClose,
}: {
  artifact: WorkGraphArtifactContent;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const graph = artifact.preview ?? artifact.workflow;
  const [activePane, setActivePane] = useState<ComparePane>("draft");
  const [source, setSource] = useState<ExecutionView | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const revision = artifact.selected_revision
    ?? (graph && "version" in graph ? graph.version : 1);
  const preview = useMemo(
    () => graph ? workGraphArtifactPreview(graph) : null,
    [graph],
  );
  const draftExecution = useMemo(
    () => preview ? projectWorkGraphWorkflowCanvasExecution(preview, revision) : null,
    [preview, revision],
  );

  const loadSource = async () => {
    if (!graph) return;
    setLoading(true);
    setFailed(false);
    try {
      setSource(await getExecutionApi(graph.source_session_key, graph.source_execution_id));
    } catch (reason: unknown) {
      console.error("[WorkGraphArtifact] source load failed", reason);
      setFailed(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSource();
    // exact source identity only changes when a different message artifact mounts.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [graph?.source_execution_id, graph?.source_session_key]);

  if (!graph || !draftExecution) return null;

  const sourcePane = (
    <CompareCanvasPanel
      badge={t("execution.workflow_artifact_source_badge")}
      title={t("execution.workflow_artifact_source")}
    >
      {loading ? (
        <div className="grid h-full place-items-center text-xs text-(--text-muted)">
          <span className="inline-flex items-center gap-2"><LoaderCircle className="h-4 w-4 animate-spin" />{t("execution.workflow_artifact_loading_source")}</span>
        </div>
      ) : source ? (
        <ExecutionWorkGraphCanvas currentId={null} directory={{}} execution={source} taskRuns={[]} />
      ) : failed ? (
        <div className="grid h-full place-items-center px-6">
          <UiResourceState
            className="w-full max-w-md min-h-0 py-5"
            impact={t("execution.workflow_artifact_source_failed_impact")}
            primaryAction={{
              icon: <RotateCcw className="h-3.5 w-3.5" />,
              label: t("execution.workflow_artifact_source_retry"),
              onClick: () => void loadSource(),
            }}
            size="sm"
            state="error"
            title={t("execution.workflow_artifact_source_failed_title")}
            urgency="polite"
            variant="card"
          />
        </div>
      ) : null}
    </CompareCanvasPanel>
  );
  const draftPane = (
    <CompareCanvasPanel
      badge={`v${revision}`}
      title={artifact.state === "saved"
        ? t("execution.workflow_artifact_saved")
        : t("execution.workflow_artifact_draft")}
    >
      <ExecutionWorkGraphCanvas currentId={null} directory={{}} execution={draftExecution} taskRuns={[]} />
    </CompareCanvasPanel>
  );

  return (
    <UiDialogPortal>
      <UiDialogBackdrop layer="dialogUnderlay" labelledBy="workgraph-compare-title" onClose={onClose}>
        <UiDialogShell size="workbench" viewport="workbench">
          <h2 className="sr-only" id="workgraph-compare-title">
            {t("execution.workflow_artifact_compare_title")}: /{graph.slash_name} · {graph.title}
          </h2>
          <UiDialogCloseButton
            className="absolute right-4 top-4 z-30 bg-(--surface-panel-background)"
            onClose={onClose}
          />
          <UiDialogBody className="flex min-h-0 flex-1 flex-col p-0">
            <div className="flex gap-1 border-b border-(--divider-subtle-color) bg-(--surface-muted-background) p-2 pr-14 lg:hidden">
              {(["source", "draft"] as const).map((pane) => (
                <button
                  aria-pressed={activePane === pane}
                  className={`flex-1 rounded-lg px-3 py-2 text-xs font-medium transition-colors ${activePane === pane ? "bg-(--surface-panel-background) text-(--text-strong) shadow-sm" : "text-(--text-muted)"}`}
                  key={pane}
                  type="button"
                  onClick={() => setActivePane(pane)}
                >
                  {pane === "source" ? t("execution.workflow_artifact_source") : t("execution.workflow_artifact_draft")}
                </button>
              ))}
            </div>
            <div className="hidden min-h-0 grid-cols-2 divide-x divide-(--divider-subtle-color) lg:grid">
              {sourcePane}
              {draftPane}
            </div>
            <div className="min-h-0 lg:hidden">{activePane === "source" ? sourcePane : draftPane}</div>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function CompareCanvasPanel({
  badge,
  children,
  title,
}: {
  badge: string;
  children: React.ReactNode;
  title: string;
}) {
  return (
    <section className="flex min-h-0 min-w-0 flex-col bg-(--surface-canvas-background)">
      <header className="shrink-0 border-b border-(--divider-subtle-color) bg-(--surface-panel-background) px-4 py-3">
        <div className="flex min-h-7 items-center justify-between gap-3 pr-10">
          <h3 className="text-sm font-semibold text-(--text-strong)">{title}</h3>
          <span className="rounded-full border border-(--divider-subtle-color) px-2 py-0.5 text-[10px] text-(--text-muted)">{badge}</span>
        </div>
      </header>
      <div className="flex min-h-[420px] min-w-0 flex-1">{children}</div>
    </section>
  );
}

function workGraphArtifactPreview(
  graph: WorkGraphWorkflowPreview | WorkGraphWorkflow,
): WorkGraphWorkflowPreview {
  if ("preview_id" in graph) return graph;
  return {
    preview_id: `saved:${graph.id}`,
    slash_name: graph.slash_name,
    title: graph.title,
    description: graph.description,
    source_execution_id: graph.source_execution_id,
    source_session_key: graph.source_session_key,
    objective: graph.objective,
    completion_criteria: graph.completion_criteria,
    nodes: graph.nodes,
    dependencies: graph.dependencies,
    expires_at: graph.updated_at,
  };
}
