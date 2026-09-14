/**
 * INPUT: owner-scoped 命名工作图目录与可选详情路由。
 * OUTPUT: 使用公共 outlined 条目的工作图目录、来源说明、创建指引、节点详情及复制/编辑/删除操作。
 * POS: “能力 > 工作图”的唯一页面入口。
 */
"use client";

import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, GitBranchPlus, Pencil, RotateCcw, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityDetailPage,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import { WorkGraphDistillationDialog } from "@/features/conversation/shared/execution/workgraph-distillation-dialog";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/lib/conversation/workgraph-workflow-events";
import { useCopyToClipboard } from "@/shared/lib/react/use-copy-to-clipboard";
import {
  getWorkGraphWorkflowsApi,
  previewSavedWorkGraphWorkflowApi,
} from "@/lib/api/conversation/execution-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { useAgentStore } from "@/store/agent";
import type { WorkGraphWorkflow, WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

import { useWorkGraphDeletion } from "./use-workgraph-deletion";
import { WorkGraphDistillationDetail } from "./workgraph-distillation-detail";

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
  const loadSequenceRef = useRef(0);
  const editorPendingRef = useRef(false);
  const editorRequestRef = useRef(0);
  useEffect(() => {
    setOpeningEditorId(null);
    return () => { editorRequestRef.current += 1; editorPendingRef.current = false; };
  }, [distillationId]);
  const { copy, copied } = useCopyToClipboard({ feedback_timeout_ms: 1800 });
  const deletion = useWorkGraphDeletion({
    onRefresh: () => setLoadRevision((current) => current + 1),
    reportFeedback: setCommandFailure,
    onDeleted: (id) => {
      loadSequenceRef.current += 1;
      setLoading(false);
      setItems((current) => current.filter((item) => item.id !== id));
      notifyCapabilitySummaryMutated({ domain: "workgraph_distillation" });
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    },
  });
  const deletionRef = useRef(deletion);
  deletionRef.current = deletion;
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
    const sequence = ++loadSequenceRef.current;
    setLoading(true);
    setLoadFailure((current) => current?.access ? current : null);
    void getWorkGraphWorkflowsApi(locale).then((next) => {
      if (active && sequence === loadSequenceRef.current) {
        setItems(next);
        setLoadedLocale(locale);
        setLoadFailure(null);
        if (!deletionRef.current.blocked) setCommandFailure(null);
        deletionRef.current.reconcile(next);
        window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
      }
    }).catch((reason: unknown) => {
      if (active && sequence === loadSequenceRef.current) setLoadFailure(getResourceFailure(reason, t("capability.workgraph_loading_failed")));
    }).finally(() => {
      if (active && sequence === loadSequenceRef.current) setLoading(false);
    });
    return () => { active = false; };
  }, [loadRevision, locale, t]);
  const hasSnapshot = loadedLocale === locale;

  const filtered = useMemo(() => {
    const search = createUiSearchMatcher(query);
    return items.filter((item) => search.matches([
      item.slash_name,
      item.title,
      item.description,
      item.objective,
      ...item.nodes.flatMap((node) => [node.subject, node.objective, node.deliverable]),
    ]));
  }, [items, query]);
  const selected = items.find((item) => item.id === distillationId) ?? null;

  const copyCommand = async (item: WorkGraphWorkflow) => {
    if (await copy(`/${item.slash_name} `)) {
      setCopiedId(item.id);
    } else {
      setCommandFailure({ title: t("capability.workgraph_copy_failed"), impact: t("capability.workgraph_copy_failed_impact"), tone: "error", onDismiss: () => setCommandFailure(null) });
    }
  };

  const openEditor = async (item: WorkGraphWorkflow) => {
    if (item.built_in || accessBlockedRef.current || editorPendingRef.current || deletion.busy || deletion.blocked) return;
    editorPendingRef.current = true;
    const request = ++editorRequestRef.current;
    setOpeningEditorId(item.id);
    setCommandFailure(null);
    try {
      if (agents.length === 0) await loadAgents();
      if (accessBlockedRef.current || request !== editorRequestRef.current) return;
      const preview = await previewSavedWorkGraphWorkflowApi(item.id, locale);
      if (accessBlockedRef.current || request !== editorRequestRef.current) return;
      setEditingWorkflowId(item.id);
      setEditingPreview(preview);
    } catch {
      if (request !== editorRequestRef.current) return;
      setCommandFailure({
        action: {
          label: t("state.reload_check"),
          onClick: () => setLoadRevision((current) => current + 1),
        },
        impact: t("capability.workgraph_edit_failure_impact"),
        onDismiss: () => setCommandFailure(null),
        title: t("capability.workgraph_edit_failed"),
        tone: "error",
      });
    } finally {
      if (request === editorRequestRef.current) {
        editorPendingRef.current = false;
        setOpeningEditorId(null);
      }
    }
  };

  const backToDirectory = () => navigate(AppRouteBuilders.workGraphDistillations());
  const staleLoadNotice = loadFailure && hasSnapshot && !loadFailure.access ? (
    <UiResourceState
      className="mb-3 min-h-0 py-3"
      impact={t("state.stale_snapshot_impact")}
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
  ) : null;
  let detailRouteContent: ReactNode = null;
  if (distillationId) {
    if (loading && !hasSnapshot) {
      detailRouteContent = (
        <CapabilityDetailPage
          backLabel={t("capability.workgraph_distillations")}
          onBack={backToDirectory}
        >
          <UiResourceState
            className="min-h-48"
            size="sm"
            state="loading"
            title={t("capability.workgraph_loading")}
          />
        </CapabilityDetailPage>
      );
    } else if (loadFailure && (loadFailure.access || !hasSnapshot)) {
      detailRouteContent = (
        <CapabilityDetailPage
          backLabel={t("capability.workgraph_distillations")}
          onBack={backToDirectory}
        >
          <UiResourceState
            className="min-h-48"
            impact={t(loadFailure.access
              ? "state.access_failure_impact"
              : "state.read_failure_impact")}
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
        </CapabilityDetailPage>
      );
    } else if (selected) {
      detailRouteContent = (
        <WorkGraphDistillationDetail
          item={selected}
          editDisabled={openingEditorId !== null || deletion.busy || deletion.blocked}
          notice={staleLoadNotice}
          onBack={backToDirectory}
          onCopy={() => void copyCommand(selected)}
          onEdit={() => void openEditor(selected)}
        />
      );
    } else {
      detailRouteContent = (
        <CapabilityDetailPage
          backLabel={t("capability.workgraph_distillations")}
          onBack={backToDirectory}
        >
          <UiResourceState
            className="min-h-48"
            primaryAction={{
              label: t("common.back"),
              onClick: backToDirectory,
            }}
            size="sm"
            state="empty"
            title={t("capability.workgraph_no_matches")}
          />
        </CapabilityDetailPage>
      );
    }
  }

  return (
    <WorkspaceSurfaceScaffold
      bodyClassName={distillationId ? "flex flex-col" : undefined}
      bodyScrollable
      stableGutter
    >
      {detailRouteContent ?? (
        <CapabilityPageLayout
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
            impact={t("state.stale_snapshot_impact")}
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
            impact={t(loadFailure.access
              ? "state.access_failure_impact"
              : "state.read_failure_impact")}
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
        ) : filtered.length === 0 ? (
          <UiResourceState
            className="min-h-48"
            description={items.length === 0 ? t("capability.workgraph_empty_description") : undefined}
            icon={<GitBranchPlus className="h-5 w-5 text-(--icon-default)" />}
            impact={items.length > 0 ? t("state.filter_impact") : undefined}
            {...(items.length > 0
              ? {
                  primaryAction: {
                    label: t("state.clear_filters"),
                    onClick: () => setQuery(""),
                  },
                }
              : { nextStep: t("capability.workgraph_empty_description") })}
            size="sm"
            state="empty"
            title={t(items.length === 0 ? "capability.workgraph_empty" : "capability.workgraph_no_matches")}
          />
        ) : (
          <div className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}>
            {filtered.map((item) => (
              <UiListRow
                variant="outlined"
                className={CAPABILITY_DIRECTORY_ROW_CLASS_NAME}
                key={item.id}
                leading={<UiSeededAvatar seed={item.slash_name} size="sm" />}
                onClick={() => navigate(AppRouteBuilders.workGraphDistillationDetail(item.id))}
                right={(
                  <div className="flex shrink-0 gap-1">
                    {!item.built_in ? (
                      <UiIconButton
                        aria-label={t("capability.workgraph_edit")}
                        disabled={openingEditorId !== null || deletion.busy || deletion.blocked}
                        onClick={(event) => {
                          event.stopPropagation();
                          void openEditor(item);
                        }}
                        size="md"
                        variant="ghost"
                      >
                        <Pencil className="h-4 w-4" />
                      </UiIconButton>
                    ) : null}
                    <UiIconButton
                      aria-label={t("capability.workgraph_copy")}
                      onClick={(event) => {
                        event.stopPropagation();
                        void copyCommand(item);
                      }}
                      size="md"
                      variant="ghost"
                    >
                      {copied && copiedId === item.id
                        ? <Check className="h-4 w-4" />
                        : <Copy className="h-4 w-4" />}
                    </UiIconButton>
                    {!item.built_in ? (
                      <UiIconButton
                        disabled={deletion.busy || deletion.blocked || openingEditorId !== null}
                        aria-label={t("execution.workflow_delete")}
                        onClick={(event) => {
                          event.stopPropagation();
                          setDeleteCandidate(item);
                        }}
                        size="md"
                        variant="ghost"
                      >
                        <Trash2 className="h-4 w-4" />
                      </UiIconButton>
                    ) : null}
                  </div>
                )}
              >
                <div className="min-w-0 flex-1">
                  <h3 className={cn(
                    "truncate",
                    getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }),
                  )}>
                    /{item.slash_name}
                  </h3>
                  <p className={cn(
                    "mt-0.5 truncate",
                    getUiTypographyClassName({ role: "metadata", tone: "muted" }),
                  )}>
                    {item.title}
                  </p>
                  <div className={cn(
                    "mt-0.5",
                    getUiTypographyClassName({ role: "caption", tone: "soft" }),
                  )}>
                    {t(item.built_in ? "capability.workgraph_builtin" : "capability.workgraph_saved")} · {item.nodes.length} {t("execution.workflow_nodes_short")}
                  </div>
                </div>
              </UiListRow>
            ))}
          </div>
          )}
        </CapabilityPageLayout>
      )}
      <ConfirmDialog
        confirmText={t("execution.workflow_delete")}
        isOpen={!accessBlocked && Boolean(deleteCandidate)}
        message={!accessBlocked && deleteCandidate ? t("execution.workflow_delete_message", { command: `/${deleteCandidate.slash_name}` }) : ""}
        onCancel={() => setDeleteCandidate(null)}
        onConfirm={() => {
          const candidate = deleteCandidate;
          setDeleteCandidate(null);
          if (accessBlockedRef.current || deletion.busy || deletion.blocked || !candidate) return;
          void deletion.remove(candidate);
        }}
        title={t("execution.workflow_delete_title")}
        variant="danger"
      />
      {!accessBlocked && editingPreview ? (
        <WorkGraphDistillationDialog
          agents={agents}
          preview={editingPreview}
          sessionKey={editingPreview.source_session_key}
          onSaved={(workflow) => {
            if (accessBlockedRef.current) return;
            setItems((current) => current.map((item) => item.id === editingWorkflowId ? workflow : item));
            notifyCapabilitySummaryMutated({ domain: "workgraph_distillation" });
          }}
          onClose={() => {
            setEditingPreview(null);
            setEditingWorkflowId(null);
          }}
        />
      ) : null}
      <FeedbackBannerViewport item={deletion.feedback ?? commandFailure} />
    </WorkspaceSurfaceScaffold>
  );
}
