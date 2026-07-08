"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Brain,
  Clock3,
  Database,
  Eraser,
  FileText,
  Loader2,
  RefreshCw,
  Trash2,
} from "lucide-react";

import {
  cleanupMemoryApi,
  deleteMemoryItemApi,
  getMemoryStatsApi,
  listMemoryItemsApi,
  searchMemoryItemsApi,
} from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import {
  formatMemoryScore,
  formatMemoryTime,
  memoryLayerKey,
  memoryScopeLabel,
  type MemoryLayerFilter,
} from "@/features/memory/memory-utils";
import {
  MemoryMetaChip,
  MemoryMetaRow,
  MemoryStatusBadge,
} from "@/features/memory/memory-ui";
import { UiIconButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiListRow } from "@/shared/ui/list-row";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";
import type { MemoryItem, MemoryStats } from "@/types/memory/memory";

interface ContactsAgentMemoryTabProps {
  agent: Agent;
}

const STATUS_OPTIONS = [
  { value: "", label: "全部状态" },
  { value: "candidate", label: "候选" },
  { value: "auto", label: "自动" },
  { value: "promoted", label: "已提升" },
  { value: "ignored", label: "已忽略" },
];

const LAYER_OPTIONS: Array<{ value: MemoryLayerFilter; label: string }> = [
  { value: "all", label: "全部层级" },
  { value: "agent", label: "Agent" },
  { value: "dm_session", label: "DM" },
  { value: "room", label: "Room" },
];

export function ContactsAgentMemoryTab({ agent }: ContactsAgentMemoryTabProps) {
  const [items, setItems] = useState<MemoryItem[]>([]);
  const [stats, setStats] = useState<MemoryStats | null>(null);
  const [selectedItemId, setSelectedItemId] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [layerFilter, setLayerFilter] = useState<MemoryLayerFilter>("all");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [deletingItemId, setDeletingItemId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const visibleItems = useMemo(() => {
    return items.filter((item) => {
      if (statusFilter && item.status !== statusFilter) {
        return false;
      }
      return layerFilter === "all" || memoryLayerKey(item.scope) === layerFilter;
    });
  }, [items, layerFilter, statusFilter]);

  const selectedItem = useMemo(
    () => visibleItems.find((item) => item.entry_id === selectedItemId) ?? visibleItems[0] ?? null,
    [selectedItemId, visibleItems],
  );

  const loadMemory = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextItems, nextStats] = await Promise.all([
        query.trim()
          ? searchMemoryItemsApi(agent.agent_id, query.trim(), 80)
          : listMemoryItemsApi(agent.agent_id, {
              limit: 120,
              status: statusFilter,
            }),
        getMemoryStatsApi(agent.agent_id),
      ]);
      setItems(nextItems);
      setStats(nextStats);
      setSelectedItemId((current) => {
        if (current && nextItems.some((item) => item.entry_id === current)) {
          return current;
        }
        return nextItems[0]?.entry_id ?? "";
      });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "加载记忆失败");
      setItems([]);
      setStats(null);
      setSelectedItemId("");
    } finally {
      setLoading(false);
    }
  }, [agent.agent_id, query, statusFilter]);

  useEffect(() => {
    void loadMemory();
  }, [loadMemory]);

  const handleDelete = useCallback(
    async (item: MemoryItem) => {
      if (!window.confirm("确定删除这条记忆？删除后不会参与召回。")) {
        return;
      }
      setDeletingItemId(item.entry_id);
      setError(null);
      try {
        await deleteMemoryItemApi(agent.agent_id, item.entry_id);
        setItems((current) => current.filter((candidate) => candidate.entry_id !== item.entry_id));
        setSelectedItemId("");
        await loadMemory();
      } catch (deleteError) {
        setError(deleteError instanceof Error ? deleteError.message : "删除记忆失败");
      } finally {
        setDeletingItemId("");
      }
    },
    [agent.agent_id, loadMemory],
  );

  const handleCleanup = useCallback(async () => {
    if (!window.confirm("清理无有效条目关联的会话摘要和检查点？")) {
      return;
    }
    setCleaning(true);
    setError(null);
    try {
      await cleanupMemoryApi(agent.agent_id);
      await loadMemory();
    } catch (cleanupError) {
      setError(cleanupError instanceof Error ? cleanupError.message : "清理记忆失败");
    } finally {
      setCleaning(false);
    }
  }, [agent.agent_id, loadMemory]);

  const statItems = useMemo(
    () => [
      { label: "总数", value: stats?.total ?? 0 },
      { label: "候选", value: stats?.candidate ?? 0 },
      { label: "自动", value: stats?.by_status?.auto ?? 0 },
      { label: "已提升", value: stats?.by_status?.promoted ?? 0 },
    ],
    [stats],
  );

  return (
    <div className="min-h-0 flex-1 overflow-hidden px-5 py-5 xl:px-6">
      <div className={cn(
        "mx-auto grid h-full min-h-0 w-full grid-cols-1 gap-4 lg:grid-cols-[390px_minmax(420px,1fr)] xl:grid-cols-[420px_minmax(480px,1fr)]",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}>
        <section className="flex min-h-0 flex-col overflow-hidden border-b border-(--divider-subtle-color) pb-3 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-3">
          <div className="flex h-12 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3.5">
            <div className="flex min-w-0 items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Brain className="h-3.5 w-3.5" />
              </span>
              <span className="truncate text-sm font-semibold text-(--text-strong)">记忆</span>
              <span className="truncate text-[11px] font-medium text-(--text-soft)">
                {visibleItems.length}/{items.length}
              </span>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <UiIconButton
                aria-label="刷新记忆"
                onClick={() => void loadMemory()}
                size="sm"
                type="button"
              >
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
              </UiIconButton>
              <UiIconButton
                aria-label="清理脏记忆"
                disabled={cleaning}
                onClick={() => void handleCleanup()}
                size="sm"
                title="清理脏记忆"
                type="button"
              >
                <Eraser className={cn("h-3.5 w-3.5", cleaning && "animate-pulse")} />
              </UiIconButton>
            </div>
          </div>

          <div className="grid grid-cols-4 gap-2 border-b border-(--divider-subtle-color) px-3.5 py-3">
            {statItems.map((stat) => (
              <div
                className="min-w-0 rounded-[10px] border border-(--divider-subtle-color) px-2 py-2"
                key={stat.label}
              >
                <div className="truncate text-[11px] font-medium leading-4 text-(--text-soft)">
                  {stat.label}
                </div>
                <div className="mt-0.5 text-[13px] font-semibold leading-5 tabular-nums text-(--text-strong)">
                  {stat.value}
                </div>
              </div>
            ))}
          </div>

          <div className="flex flex-col gap-2 border-b border-(--divider-subtle-color) p-3">
            <UiSearchInput
              controlSize="sm"
              onChange={setQuery}
              placeholder="搜索记忆"
              value={query}
            />
            <div className="grid grid-cols-2 gap-2">
              <UiSelectMenu
                ariaLabel="筛选记忆状态"
                onChange={setStatusFilter}
                options={STATUS_OPTIONS}
                size="sm"
                value={statusFilter}
              />
              <UiSelectMenu
                ariaLabel="筛选记忆层级"
                onChange={(value) => setLayerFilter(value as MemoryLayerFilter)}
                options={LAYER_OPTIONS}
                size="sm"
                value={layerFilter}
              />
            </div>
          </div>

          <MemoryItemList
            error={error}
            isLoading={loading}
            items={visibleItems}
            onSelect={setSelectedItemId}
            selectedItemId={selectedItem?.entry_id ?? ""}
          />
        </section>

        <MemoryItemInspector
          isDeleting={selectedItem ? deletingItemId === selectedItem.entry_id : false}
          item={selectedItem}
          onDelete={handleDelete}
        />
      </div>
    </div>
  );
}

function MemoryItemList({
  error,
  isLoading: isLoading,
  items,
  onSelect: onSelect,
  selectedItemId: selectedItemId,
}: {
  error: string | null;
  isLoading: boolean;
  items: MemoryItem[];
  onSelect: (entryId: string) => void;
  selectedItemId: string;
}) {
  if (isLoading && items.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-soft)">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="min-h-0 flex-1 px-3 pt-4">
        <UiStateBlock
          description={error}
          size="sm"
          title="记忆加载失败"
          tone="danger"
          variant="plain"
        />
      </div>
    );
  }
  if (items.length === 0) {
    return (
      <div className="min-h-0 flex-1 px-3 pt-4">
        <UiStateBlock
          description="当前筛选条件下没有记忆条目。"
          size="sm"
          title="暂无记忆"
          variant="plain"
        />
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {items.map((item) => {
        const active = item.entry_id === selectedItemId;
        return (
          <UiListRow
            active={active}
            className="min-h-[112px] rounded-none border-b border-(--divider-subtle-color) px-3.5 py-3"
            key={item.entry_id}
            onClick={() => onSelect(item.entry_id)}
          >
            <div className="flex min-w-0 items-center gap-2">
              <span className="min-w-0 flex-1 truncate text-[13px] font-semibold leading-5 text-(--text-strong)">
                {item.title || item.entry_id}
              </span>
              <MemoryStatusBadge status={item.status} />
            </div>
            <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
              <MemoryMetaChip>{memoryScopeLabel(item.scope)}</MemoryMetaChip>
              {item.kind ? <MemoryMetaChip>{item.kind}</MemoryMetaChip> : null}
              {item.score !== undefined ? <MemoryMetaChip>{formatMemoryScore(item.score)}</MemoryMetaChip> : null}
              <MemoryMetaChip>access {item.access_count}</MemoryMetaChip>
              {item.created_at ? <MemoryMetaChip>{formatMemoryTime(item.created_at)}</MemoryMetaChip> : null}
            </div>
            <p className="mt-2 line-clamp-2 whitespace-pre-wrap text-xs leading-5 text-(--text-default)">
              {item.content}
            </p>
          </UiListRow>
        );
      })}
    </div>
  );
}

function MemoryItemInspector({
  isDeleting: isDeleting,
  item,
  onDelete: onDelete,
}: {
  isDeleting: boolean;
  item: MemoryItem | null;
  onDelete: (item: MemoryItem) => void;
}) {
  if (!item) {
    return (
      <section className="flex min-h-0 items-center justify-center px-6 text-xs text-(--text-soft)">
        未选择记忆
      </section>
    );
  }

  const rawFields = (item.fields ?? []).filter((field) => field.value.trim() !== "");

  return (
    <section className="flex min-h-0 flex-col overflow-hidden">
      <div className="flex min-h-14 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
            <FileText className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold leading-5 text-(--text-strong)">
              {item.title || item.entry_id}
            </div>
            <div className="mt-0.5 truncate text-[11px] font-medium leading-4 text-(--text-soft)">
              {item.entry_id}
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <MemoryStatusBadge status={item.status} />
          <UiIconButton
            aria-label="删除记忆"
            disabled={isDeleting}
            onClick={() => onDelete(item)}
            size="sm"
            title="删除记忆"
            tone="danger"
            type="button"
          >
            {isDeleting ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Trash2 className="h-3.5 w-3.5" />
            )}
          </UiIconButton>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
        <section className="border-b border-(--divider-subtle-color) pb-4">
          <div className="text-[11px] font-semibold leading-4 text-(--text-soft)">记忆内容</div>
          <p className="mt-2 whitespace-pre-wrap text-[14px] leading-7 text-(--text-strong)">
            {item.content}
          </p>
        </section>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <MemoryMetaChip>
            <Database className="h-3 w-3" />
            {memoryScopeLabel(item.scope)}
          </MemoryMetaChip>
          {item.kind ? <MemoryMetaChip>{item.kind}</MemoryMetaChip> : null}
          {item.category ? <MemoryMetaChip>{item.category}</MemoryMetaChip> : null}
          {item.priority ? <MemoryMetaChip>{item.priority}</MemoryMetaChip> : null}
          {item.created_at ? (
            <MemoryMetaChip>
              <Clock3 className="h-3 w-3" />
              {formatMemoryTime(item.created_at)}
            </MemoryMetaChip>
          ) : null}
          <MemoryMetaChip>access {item.access_count}</MemoryMetaChip>
          {item.score !== undefined ? <MemoryMetaChip>{formatMemoryScore(item.score)}</MemoryMetaChip> : null}
        </div>

        <dl className="mt-4 grid gap-1.5 border-t border-(--divider-subtle-color) pt-3 text-[11px] leading-5">
          <MemoryMetaRow label="scope" value={item.scope} />
          <MemoryMetaRow label="source" value={item.source} />
          <MemoryMetaRow label="path" value={item.path} />
          <MemoryMetaRow label="session" value={item.session_key} />
          <MemoryMetaRow label="round" value={item.round_id} />
        </dl>

        {rawFields.length > 0 ? (
          <details className="mt-4 border-t border-(--divider-subtle-color) pt-3 text-[11px] leading-5 text-(--text-soft)">
            <summary className="cursor-pointer select-none font-medium">
              原始字段 {rawFields.length}
            </summary>
            <dl className="mt-2 grid gap-1.5">
              {rawFields.map((field) => (
                <MemoryMetaRow
                  key={`${field.key}:${field.value}`}
                  label={field.key}
                  value={field.value}
                />
              ))}
            </dl>
          </details>
        ) : null}
      </div>
    </section>
  );
}
