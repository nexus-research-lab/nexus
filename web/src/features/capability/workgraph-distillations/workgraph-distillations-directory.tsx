/**
 * INPUT: owner-scoped 命名工作图目录与可选详情路由。
 * OUTPUT: 与 Loop 同构的能力目录、抽象节点详情、复制与删除操作。
 * POS: “能力 > 工作图”的唯一页面入口。
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import { Check, Copy, GitBranchPlus, Trash2 } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import { notifyCapabilitySummaryMutated } from "@/features/capability/capability-summary-events";
import { NamedWorkGraphSketch } from "@/features/conversation/shared/execution/named-workgraph-sketch";
import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "@/features/conversation/shared/execution/workgraph-distillation-intent";
import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import {
  deleteWorkGraphWorkflowApi,
  getWorkGraphWorkflowsApi,
} from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { UiListRow } from "@/shared/ui/list/list-row";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

export function WorkGraphDistillationsDirectory() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { distillationId } = useParams<{ distillationId?: string }>();
  const [items, setItems] = useState<WorkGraphWorkflow[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<WorkGraphWorkflow | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    void getWorkGraphWorkflowsApi().then((next) => {
      if (active) {
        setItems(next);
        window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
      }
    }).catch((reason: unknown) => {
      if (active) setError(getErrorMessage(reason, t("capability.workgraph_loading_failed")));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [t]);

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

  return (
    <WorkspaceSurfaceScaffold bodyScrollable stableGutter>
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
        <CapabilitySectionHeader
          count={t("capability.result_count", { count: filtered.length })}
          title={t("capability.workgraph_distillations")}
        />
        {loading ? (
          <div className="py-10 text-sm text-(--text-muted)">{t("capability.workgraph_loading")}</div>
        ) : error ? (
          <div className="py-10 text-sm text-(--destructive)">{error}</div>
        ) : selected ? (
          <WorkGraphDistillationDetail
            item={selected}
            onBack={() => navigate(AppRouteBuilders.workGraphDistillations())}
            onCopy={() => void copyCommand(selected)}
          />
        ) : filtered.length === 0 ? (
          <UiStateBlock
            className="min-h-48"
            description={t("capability.workgraph_empty_description")}
            icon={<GitBranchPlus className="h-5 w-5 text-(--icon-default)" />}
            size="sm"
            title={t("capability.workgraph_empty")}
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
                      <UiIconButton aria-label={t("capability.workgraph_copy")} onClick={(event) => { event.stopPropagation(); void copyCommand(item); }} size="md" variant="ghost">
                        {copiedId === item.id ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      </UiIconButton>
                      <UiIconButton aria-label={t("execution.workflow_delete")} onClick={(event) => { event.stopPropagation(); setDeleteCandidate(item); }} size="md" variant="ghost">
                        <Trash2 className="h-4 w-4" />
                      </UiIconButton>
                    </div>
                  )}
                >
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-[14px] font-medium text-(--text-strong)">/{item.slash_name}</h3>
                    <p className="mt-0.5 truncate text-compact text-(--text-muted)">{item.title}</p>
                    <div className="mt-0.5 text-2xs text-(--text-soft)">{item.nodes.length} {t("execution.workflow_nodes_short")}</div>
                  </div>
                </UiListRow>
            ))}
          </div>
        )}
      </CapabilityPageLayout>
      <ConfirmDialog
        confirmText={t("execution.workflow_delete")}
        isOpen={Boolean(deleteCandidate)}
        message={deleteCandidate ? t("execution.workflow_delete_message", { command: `/${deleteCandidate.slash_name}` }) : ""}
        onCancel={() => setDeleteCandidate(null)}
        onConfirm={() => {
          const candidate = deleteCandidate;
          setDeleteCandidate(null);
          if (!candidate) return;
          void deleteWorkGraphWorkflowApi(candidate.id).then(() => {
            setItems((current) => current.filter((item) => item.id !== candidate.id));
            notifyCapabilitySummaryMutated({ domain: "workgraph_distillation" });
            window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
          }).catch((reason: unknown) => setError(getErrorMessage(reason, t("capability.workgraph_delete_failed"))));
        }}
        title={t("execution.workflow_delete_title")}
        variant="danger"
      />
    </WorkspaceSurfaceScaffold>
  );
}

function WorkGraphDistillationDetail({ item, onBack, onCopy }: { item: WorkGraphWorkflow; onBack: () => void; onCopy: () => void }) {
  const { t } = useI18n();
  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <button className="text-xs text-(--text-muted) hover:text-(--text-strong)" onClick={onBack} type="button">← {t("common.back")}</button>
          <h2 className="mt-2 text-lg font-semibold text-(--text-strong)">/{item.slash_name}</h2>
          <p className="mt-1 text-sm text-(--text-muted)">{item.description}</p>
        </div>
        <button className="rounded-[8px] bg-(--brand-action) px-3 py-2 text-xs font-semibold text-white" onClick={onCopy} type="button">{t("capability.workgraph_copy")}</button>
      </div>
      <div className="rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-muted-background) p-3 text-sm text-(--text-default)">{item.objective}</div>
      <NamedWorkGraphSketch dependencies={item.dependencies} nodes={item.nodes} />
    </div>
  );
}
