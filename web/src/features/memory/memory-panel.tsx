"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Check,
  Database,
  Eraser,
  RefreshCw,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";

import {
  addUserMemoryItemApi,
  cleanupUserMemoryApi,
  deleteUserMemoryItemApi,
  getUserMemoryStatsApi,
  ignoreUserMemoryItemApi,
  listUserMemoryItemsApi,
  promoteUserMemoryItemApi,
  searchUserMemoryItemsApi,
  updateUserMemoryItemApi,
} from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import {
  formatMemoryScore,
  formatMemoryTime,
  memoryScopeLabel,
} from "@/features/memory/memory-utils";
import { MemoryMetaChip, MemoryStatusBadge } from "@/features/memory/memory-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { MemoryItem, MemoryStats } from "@/types/memory/memory";

type FeedbackTone = "success" | "error" | "warning";

interface FeedbackState {
  tone: FeedbackTone;
  message: string;
}

const STATUS_OPTIONS = [
  { value: "", label: "全部" },
  { value: "candidate", label: "候选" },
  { value: "auto", label: "自动" },
  { value: "promoted", label: "已提升" },
  { value: "ignored", label: "已忽略" },
];

export function MemoryPanel() {
  const { t } = useI18n();
  const [items, setItems] = useState<MemoryItem[]>([]);
  const [stats, setStats] = useState<MemoryStats | null>(null);
  const [status, setStatus] = useState("");
  const [query, setQuery] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newContent, setNewContent] = useState("");
  const [editingId, setEditingId] = useState("");
  const [editingContent, setEditingContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [mutatingId, setMutatingId] = useState("");
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [nextItems, nextStats] = await Promise.all([
        query.trim()
          ? searchUserMemoryItemsApi(query.trim(), 100)
          : listUserMemoryItemsApi({ limit: 200, status }),
        getUserMemoryStatsApi(),
      ]);
      setItems(nextItems);
      setStats(nextStats);
    } catch (error) {
      setFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : "刷新记忆失败",
      });
    } finally {
      setLoading(false);
    }
  }, [query, status]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleAdd = async () => {
    if (!newContent.trim()) {
      return;
    }
    setLoading(true);
    try {
      await addUserMemoryItemApi({
        title: newTitle.trim(),
        content: newContent.trim(),
        kind: "LRN",
        category: "preference",
        status: "candidate",
        priority: "medium",
        source: "manual",
      });
      setNewTitle("");
      setNewContent("");
      setFeedback({ tone: "success", message: "记忆已加入候选区" });
      await refresh();
    } catch (error) {
      setFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : "新增记忆失败",
      });
    } finally {
      setLoading(false);
    }
  };

  const mutateItem = async (
    item: MemoryItem,
    action: "promote" | "ignore" | "delete" | "save",
  ) => {
    if (action === "delete" && !window.confirm("确定删除这条记忆？删除后不会参与召回。")) {
      return;
    }
    setMutatingId(item.entry_id);
    try {
      if (action === "promote") {
        await promoteUserMemoryItemApi(item.entry_id, "memory");
        setFeedback({ tone: "success", message: "记忆已提升到 MEMORY.md" });
      } else if (action === "ignore") {
        await ignoreUserMemoryItemApi(item.entry_id);
        setFeedback({ tone: "success", message: "候选记忆已忽略" });
      } else if (action === "delete") {
        await deleteUserMemoryItemApi(item.entry_id);
        setFeedback({ tone: "success", message: "记忆已删除" });
      } else {
        await updateUserMemoryItemApi(item.entry_id, {
          content: editingContent,
        });
        setEditingId("");
        setEditingContent("");
        setFeedback({ tone: "success", message: "记忆已保存" });
      }
      await refresh();
    } catch (error) {
      setFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : "操作失败",
      });
    } finally {
      setMutatingId("");
    }
  };

  const handleCleanup = async () => {
    if (!window.confirm("清理无有效条目关联的会话摘要和检查点？")) {
      return;
    }
    setCleaning(true);
    try {
      const result = await cleanupUserMemoryApi();
      setFeedback({
        tone: "success",
        message: `已清理 ${result.removed_session_files + result.removed_checkpoints + result.removed_empty_diaries} 项脏数据`,
      });
      await refresh();
    } catch (error) {
      setFeedback({
        tone: "error",
        message: error instanceof Error ? error.message : "清理记忆失败",
      });
    } finally {
      setCleaning(false);
    }
  };

  const statItems: Array<[string, number]> = [
    ["总数", stats?.total ?? 0],
    ["候选", stats?.candidate ?? 0],
    ["已访问", stats?.accessed ?? 0],
    ["检查点", stats?.checkpointed ?? 0],
  ];

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      header={
        <WorkspaceSurfaceHeader
          badge={t("capability.memory_badge", { count: stats?.total ?? items.length })}
          density="compact"
          leading={<Database className="h-4 w-4" />}
          subtitle={t("capability.memory_subtitle")}
          title={t("capability.memory")}
          trailing={
            <>
              <WorkspaceSurfaceToolbarAction disabled={loading} onClick={refresh}>
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction disabled={cleaning} onClick={handleCleanup}>
                <Eraser className={cn("h-3.5 w-3.5", cleaning && "animate-pulse")} />
                清理
              </WorkspaceSurfaceToolbarAction>
            </>
          }
        />
      }
      stableGutter
    >
      <CapabilityPageLayout
        description={t("capability.memory_intro_description")}
        title={t("capability.memory_intro_title")}
      >
        <CapabilityFilterBar>
          <CapabilityFilterSearchInput
            onChange={setQuery}
            placeholder={t("capability.memory_search_placeholder")}
            value={query}
          />
          <CapabilityFilterSelect
            ariaLabel={t("capability.memory_filter_status_aria")}
            onChange={setStatus}
            options={STATUS_OPTIONS}
            value={status}
          />
        </CapabilityFilterBar>

        <CapabilitySectionHeader title={t("capability.memory_overview_title")} />
        <section className="mb-5 grid gap-3 sm:grid-cols-4">
          {statItems.map(([label, value]) => (
            <div
              className="min-w-0 rounded-[12px] border border-(--divider-subtle-color) px-3 py-2.5"
              key={label}
            >
              <div className="text-[11px] font-medium text-(--text-soft)">{label}</div>
              <div className="mt-1 text-base font-semibold tabular-nums text-(--text-strong)">{value}</div>
            </div>
          ))}
        </section>

        <section className="rounded-[12px] border border-(--divider-subtle-color) px-3 py-3">
          <div className="mb-2 flex items-end justify-between gap-3">
            <div className="min-w-0">
              <h2 className="text-[14px] font-semibold text-(--text-strong)">新增候选记忆</h2>
              <p className="mt-0.5 text-[12px] leading-5 text-(--text-soft)">
                写入候选区后可继续编辑、忽略或提升到 MEMORY.md。
              </p>
            </div>
          </div>
          <div className="grid gap-2 md:grid-cols-[220px_1fr_auto]">
            <UiInput
              onChange={(event) => setNewTitle(event.target.value)}
              placeholder="标题"
              value={newTitle}
              variant="surface"
            />
            <UiInput
              onChange={(event) => setNewContent(event.target.value)}
              placeholder="新增候选记忆"
              value={newContent}
              variant="surface"
            />
            <UiButton
              disabled={!newContent.trim() || loading}
              onClick={handleAdd}
              type="button"
            >
              <Check className="h-3.5 w-3.5" />
              添加
            </UiButton>
          </div>
        </section>

        <section className="mt-4 overflow-hidden rounded-[12px] border border-(--divider-subtle-color)">
          <div className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-3">
            <h2 className="text-[14px] font-semibold text-(--text-strong)">记忆条目</h2>
            <span className="text-[12px] font-medium text-(--text-soft)">{items.length}</span>
          </div>
          {items.length === 0 ? (
            <UiStateBlock description="当前筛选条件下没有可管理的记忆条目。" size="sm" title="暂无记忆条目" />
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {items.map((item) => {
                const isEditing = editingId === item.entry_id;
                const isMutating = mutatingId === item.entry_id;
                return (
                  <article className="grid min-h-[132px] gap-3 px-4 py-3 md:grid-cols-[1fr_auto]" key={item.entry_id}>
                    <div className="min-w-0">
                      <div className="flex min-w-0 flex-wrap items-center gap-2">
                        <span className="truncate text-[13px] font-semibold leading-5 text-(--text-strong)">
                          {item.title || item.entry_id}
                        </span>
                        <MemoryStatusBadge status={item.status} />
                        {item.priority ? (
                          <span className="text-[11px] text-(--text-soft)">{item.priority}</span>
                        ) : null}
                      </div>
                      <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
                        {item.kind ? <MemoryMetaChip>{item.kind}</MemoryMetaChip> : null}
                        {item.category ? <MemoryMetaChip>{item.category}</MemoryMetaChip> : null}
                        {item.scope ? <MemoryMetaChip>{memoryScopeLabel(item.scope)}</MemoryMetaChip> : null}
                        {item.source ? <MemoryMetaChip>{item.source}</MemoryMetaChip> : null}
                        {item.created_at ? <MemoryMetaChip>{formatMemoryTime(item.created_at)}</MemoryMetaChip> : null}
                        <MemoryMetaChip>access {item.access_count}</MemoryMetaChip>
                        {item.score !== undefined ? <MemoryMetaChip>{formatMemoryScore(item.score)}</MemoryMetaChip> : null}
                      </div>
                      {isEditing ? (
                        <UiTextarea
                          className="mt-2"
                          controlSize="md"
                          onChange={(event) => setEditingContent(event.target.value)}
                          value={editingContent}
                          variant="surface"
                        />
                      ) : (
                        <p className="mt-2 line-clamp-3 whitespace-pre-wrap text-[12px] leading-5 text-(--text-default)">
                          {item.content}
                        </p>
                      )}
                    </div>
                    <div className="flex items-start gap-1 md:pt-0.5">
                      {isEditing ? (
                        <>
                          <UiIconButton
                            disabled={isMutating}
                            onClick={() => void mutateItem(item, "save")}
                            size="md"
                            title="保存"
                            type="button"
                          >
                            <Check className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            onClick={() => setEditingId("")}
                            size="md"
                            title="取消"
                            type="button"
                          >
                            <X className="h-3.5 w-3.5" />
                          </UiIconButton>
                        </>
                      ) : (
                        <>
                          <UiIconButton
                            disabled={isMutating}
                            onClick={() => {
                              setEditingId(item.entry_id);
                              setEditingContent(item.content);
                            }}
                            size="md"
                            title="编辑"
                            type="button"
                          >
                            <Database className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={isMutating}
                            onClick={() => void mutateItem(item, "promote")}
                            size="md"
                            title="提升"
                            type="button"
                          >
                            <ShieldCheck className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={isMutating}
                            onClick={() => void mutateItem(item, "ignore")}
                            size="md"
                            title="忽略"
                            type="button"
                          >
                            <X className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={isMutating}
                            onClick={() => void mutateItem(item, "delete")}
                            size="md"
                            title="删除"
                            tone="danger"
                            type="button"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </UiIconButton>
                        </>
                      )}
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </section>
      </CapabilityPageLayout>
      <FeedbackBannerStack
        items={feedback ? [
          {
            key: "memory-feedback",
            message: feedback.message,
            onDismiss: () => setFeedback(null),
            title: feedback.tone === "error" ? "操作失败" : feedback.tone === "warning" ? "需要注意" : "操作完成",
            tone: feedback.tone,
          },
        ] : []}
      />
    </WorkspaceSurfaceScaffold>
  );
}
