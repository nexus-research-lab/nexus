/**
 * INPUT: owner-scoped 命名工作图目录与可选详情路由。
 * OUTPUT: 带来源说明与创建指引的工作图目录、节点详情、复制、编辑与删除操作。
 * POS: “能力 > 工作图”的唯一页面入口。
 */
"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, GitBranchPlus, Pencil, RotateCcw, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import { WorkGraphMetadataEditorDialog } from "@/features/conversation/shared/execution/workgraph-metadata-editor-dialog";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/features/conversation/shared/execution/workgraph-distillation-intent";
import { WorkGraphWorkflowCanvasPreview } from "@/features/conversation/shared/execution/workgraph-workflow-canvas-preview";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import {
  deleteWorkGraphWorkflowApi,
  getWorkGraphWorkflowsApi,
  previewSavedWorkGraphWorkflowApi,
  scheduleWorkGraphWorkflowSaveApi,
} from "@/lib/api/conversation/execution-api";
import {
  getErrorMessage,
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiListRow } from "@/shared/ui/list/list-row";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { useAgentStore } from "@/store/agent";
import type { WorkGraphWorkflow, WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

export function WorkGraphDistillationsDirectory() {
  const { locale, t } = useI18n();
  const navigate = useNavigate();
  const { distillationId } = useParams<{ distillationId?: string }>();
  const [items, setItems] = useState<WorkGraphWorkflow[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadFailure, setLoadFailure] = useState<ResourceFailure | null>(null);
  const [loadedLocale, setLoadedLocale] = useState<string | null>(null);
  const [loadRevision, setLoadRevision] = useState(0);
  const [commandFailure, setCommandFailure] = useState<FeedbackBannerProps | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<WorkGraphWorkflow | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [editingWorkflowId, setEditingWorkflowId] = useState<string | null>(null);
  const [editingPreview, setEditingPreview] = useState<WorkGraphWorkflowPreview | null>(null);
  const [openingEditorId, setOpeningEditorId] = useState<string | null>(null);
  const agents = useAgentStore((state) => state.agents);
  const loadAgents = useAgentStore((state) => state.load_agents_from_server);
  const accessBlocked = Boolean(loadFailure?.access);
  const accessBlockedRef = useRef(accessBlocked);
  accessBlockedRef.current = accessBlocked;

  useEffect(() => {
    if (!accessBlocked) return;
    setDeleteCandidate(null);
    setEditingPreview(null);
    setEditingWorkflowId(null);
    setOpeningEditorId(null);
  }, [accessBlocked]);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadFailure((current) => current?.access ? current : null);
    void getWorkGraphWorkflowsApi(locale).then((next) => {
      if (active) {
        setItems(next);
        setLoadedLocale(locale);
        setLoadFailure(null);
        setCommandFailure(null);
        window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
      }
    }).catch((reason: unknown) => {
      if (active) setLoadFailure(getResourceFailure(reason, t("capability.workgraph_loading_failed")));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [loadRevision, locale, t]);
  const hasSnapshot = loadedLocale === locale;

  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    if (!normalized) return items;
    return items.filter((item) => [
      item.slash_name,
      item.title,
      item.description ?? "",
      item.objective,
      ...item.nodes.flatMap((node) => [node.subject, node.objective, node.deliverable]),
    ].join(" ").toLocaleLowerCase().includes(normalized));
  }, [items, query]);
  const selected = items.find((item) => item.id === distillationId) ?? null;

  const copyCommand = async (item: WorkGraphWorkflow) => {
    await writeTextToClipboard(`/${item.slash_name} `);
    setCopiedId(item.id);
    window.setTimeout(() => setCopiedId((current) => current === item.id ? null : current), 1800);
  };

  const openEditor = async (item: WorkGraphWorkflow) => {
    if (accessBlockedRef.current || openingEditorId) return;
    setOpeningEditorId(item.id);
    setCommandFailure(null);
    try {
      if (agents.length === 0) await loadAgents();
      if (accessBlockedRef.current) return;
      const preview = await previewSavedWorkGraphWorkflowApi(item.id, locale);
      if (accessBlockedRef.current) return;
      setEditingWorkflowId(item.id);
      setEditingPreview(preview);
    } catch (reason: unknown) {
      setCommandFailure({
        action: {
          label: t("state.reload_check"),
          onClick: () => setLoadRevision((current) => current + 1),
        },
        impact: t("capability.workgraph_edit_failure_impact"),
        message: getErrorMessage(reason, t("capability.workgraph_edit_failed")),
        nextStep: t("capability.workgraph_edit_failure_next_step"),
        onDismiss: () => setCommandFailure(null),
        title: t("capability.workgraph_edit_failed"),
        tone: "error",
      });
    } finally {
      setOpeningEditorId(null);
    }
  };

  return (
    <WorkspaceSurfaceScaffold
      bodyClassName={selected ? "flex flex-col" : undefined}
      bodyScrollable
      stableGutter
    >
      <CapabilityPageLayout
        className={selected ? "flex min-h-full flex-1 flex-col" : undefined}
        description={t("capability.workgraph_intro_description")}
        title={t("capability.workgraph_intro_title")}
      >
        <CapabilityFilterBar>
          <CapabilityFilterSearchInput
            onChange={setQuery}
            placeholder={t("capability.workgraph_search_placeholder")}
            value={query}
          />
        </CapabilityFilterBar>
        {loadFailure && hasSnapshot && !loadFailure.access ? (
          <UiResourceState
            className="mb-3 min-h-0 py-3"
            description={loadFailure.message}
            impact={t("state.stale_snapshot_impact")}
            nextStep={t("state.retry_next_step")}
            primaryAction={{
              icon: <RotateCcw className="h-3.5 w-3.5" />,
              label: t("state.retry"),
              onClick: () => setLoadRevision((current) => current + 1),
            }}
            role="status"
            size="sm"
            state="error"
            title={t("capability.workgraph_loading_failed")}
          />
        ) : null}
        {loading && !hasSnapshot ? (
          <UiResourceState
            className="min-h-48"
            size="sm"
            state="loading"
            title={t("capability.workgraph_loading")}
          />
        ) : loadFailure && (loadFailure.access || !hasSnapshot) ? (
          <UiResourceState
            className="min-h-48"
            description={loadFailure.message}
            impact={t(loadFailure.access
              ? "state.access_failure_impact"
              : "state.read_failure_impact")}
            nextStep={t(loadFailure.access
              ? "state.permission_next_step"
              : "state.retry_next_step")}
            primaryAction={{
              icon: <RotateCcw className="h-3.5 w-3.5" />,
              label: t("state.retry"),
              onClick: () => setLoadRevision((current) => current + 1),
            }}
            size="sm"
            state="error"
            title={t(loadFailure.access
              ? "state.permission_title"
              : "capability.workgraph_loading_failed")}
          />
        ) : selected ? (
          <WorkGraphDistillationDetail
            item={selected}
            onBack={() => navigate(AppRouteBuilders.workGraphDistillations())}
            onCopy={() => void copyCommand(selected)}
            onEdit={() => void openEditor(selected)}
          />
        ) : filtered.length === 0 ? (
          <UiResourceState
            className="min-h-48"
            description={items.length === 0 ? t("capability.workgraph_empty_description") : undefined}
            icon={<GitBranchPlus className="h-5 w-5 text-(--icon-default)" />}
            impact={items.length > 0 ? t("state.filter_impact") : undefined}
            nextStep={items.length > 0
              ? t("state.clear_filters_next_step")
              : t("capability.workgraph_empty_description")}
            primaryAction={items.length > 0 ? {
              label: t("state.clear_filters"),
              onClick: () => setQuery(""),
            } : undefined}
            size="sm"
            state="empty"
            title={t(items.length === 0 ? "capability.workgraph_empty" : "capability.workgraph_no_matches")}
          />
        ) : (
          <div className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}>
            {filtered.map((item) => (
                <UiListRow
                  className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
                  key={item.id}
                  leading={<UiSeededAvatar seed={item.slash_name} size="sm" />}
                  onClick={() => navigate(AppRouteBuilders.workGraphDistillationDetail(item.id))}
                  right={(
                    <div className="flex shrink-0 gap-1">
                      {!item.built_in ? (
                        <UiIconButton aria-label={t("capability.workgraph_edit")} disabled={openingEditorId !== null} onClick={(event) => { event.stopPropagation(); void openEditor(item); }} size="md" variant="ghost">
                          <Pencil className="h-4 w-4" />
                        </UiIconButton>
                      ) : null}
                      <UiIconButton aria-label={t("capability.workgraph_copy")} onClick={(event) => { event.stopPropagation(); void copyCommand(item); }} size="md" variant="ghost">
                        {copiedId === item.id ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      </UiIconButton>
                      {!item.built_in ? (
                        <UiIconButton aria-label={t("execution.workflow_delete")} onClick={(event) => { event.stopPropagation(); setDeleteCandidate(item); }} size="md" variant="ghost">
                          <Trash2 className="h-4 w-4" />
                        </UiIconButton>
                      ) : null}
                    </div>
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-base font-medium text-(--text-strong)">/{item.slash_name}</h3>
                    <p className="mt-0.5 truncate text-compact text-(--text-muted)">{item.title}</p>
                    <div className="mt-0.5 text-2xs text-(--text-soft)">
                      {t(item.built_in ? "capability.workgraph_builtin" : "capability.workgraph_saved")} · {item.nodes.length} {t("execution.workflow_nodes_short")}
                    </div>
                  </div>
                </UiListRow>
            ))}
          </div>
        )}
      </CapabilityPageLayout>
      <ConfirmDialog
        confirmText={t("execution.workflow_delete")}
        isOpen={!accessBlocked && Boolean(deleteCandidate)}
        message={!accessBlocked && deleteCandidate ? t("execution.workflow_delete_message", { command: `/${deleteCandidate.slash_name}` }) : ""}
        onCancel={() => setDeleteCandidate(null)}
        onConfirm={() => {
          const candidate = deleteCandidate;
          setDeleteCandidate(null);
          if (accessBlockedRef.current || !candidate) return;
          void deleteWorkGraphWorkflowApi(candidate.id).then(() => {
            setCommandFailure(null);
            setItems((current) => current.filter((item) => item.id !== candidate.id));
            notifyCapabilitySummaryMutated({ domain: "workgraph_distillation" });
            window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
          }).catch((reason: unknown) => setCommandFailure({
            action: {
              label: t("state.reload_check"),
              onClick: () => setLoadRevision((current) => current + 1),
            },
            impact: t("capability.workgraph_delete_failure_impact"),
            message: getErrorMessage(reason, t("capability.workgraph_delete_failed")),
            nextStep: t("capability.workgraph_delete_failure_next_step"),
            onDismiss: () => setCommandFailure(null),
            title: t("capability.workgraph_delete_failed"),
            tone: "error",
          }));
        }}
        title={t("execution.workflow_delete_title")}
        variant="danger"
      />
      {!accessBlocked && editingPreview ? (
        <WorkGraphMetadataEditorDialog
          agents={agents}
          preview={editingPreview}
          sessionKey={editingPreview.source_session_key}
          onApply={async (nextPreview) => {
            if (accessBlockedRef.current) return;
            await scheduleWorkGraphWorkflowSaveApi(nextPreview.source_session_key, nextPreview.preview_id, {
              description: nextPreview.description,
              slash_name: nextPreview.slash_name,
              title: nextPreview.title,
            });
            const workflowId = editingWorkflowId;
            if (workflowId) {
              setItems((current) => current.map((item) => item.id === workflowId ? {
                ...item,
                completion_criteria: nextPreview.completion_criteria,
                dependencies: nextPreview.dependencies,
                description: nextPreview.description,
                nodes: nextPreview.nodes,
                objective: nextPreview.objective,
                slash_name: nextPreview.slash_name,
                title: nextPreview.title,
                // The save is scheduled asynchronously; keep the persisted aggregate
                // version until the refreshed directory confirms the new revision.
                version: item.version,
              } : item));
            }
            setEditingPreview(null);
            setEditingWorkflowId(null);
          }}
          onClose={() => {
            setEditingPreview(null);
            setEditingWorkflowId(null);
          }}
        />
      ) : null}
      <FeedbackBannerViewport item={commandFailure} />
    </WorkspaceSurfaceScaffold>
  );
}

function WorkGraphDistillationDetail({ item, onBack, onCopy, onEdit }: { item: WorkGraphWorkflow; onBack: () => void; onCopy: () => void; onEdit: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex shrink-0 items-start justify-between gap-3">
        <div>
          <button className="text-xs text-(--text-muted) hover:text-(--text-strong)" onClick={onBack} type="button">← {t("common.back")}</button>
          <h2 className="mt-2 text-lg font-semibold text-(--text-strong)">/{item.slash_name}</h2>
          <p className="mt-1 text-sm text-(--text-muted)">{item.description}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {!item.built_in ? <button className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-control-background) px-3 py-2 text-xs font-semibold text-(--text-strong)" onClick={onEdit} type="button">{t("capability.workgraph_edit")}</button> : null}
          <button className="rounded-[8px] bg-(--brand-action) px-3 py-2 text-xs font-semibold text-white" onClick={onCopy} type="button">{t("capability.workgraph_copy")}</button>
        </div>
      </div>
      <div className="shrink-0 rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-muted-background) p-3 text-sm text-(--text-default)">{item.objective}</div>
      <WorkGraphWorkflowCanvasPreview
        className="min-h-[360px] flex-1 overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-(--surface-canvas-background)"
        workflow={item}
      />
    </div>
  );
}
