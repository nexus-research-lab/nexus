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
  cleanup_memory_api,
  delete_memory_item_api,
  get_memory_stats_api,
  list_memory_items_api,
  search_memory_items_api,
} from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import {
  format_memory_score,
  format_memory_time,
  memory_layer_key,
  memory_layer_label,
  type MemoryLayerFilter,
} from "@/features/memory/memory-utils";
import {
  MemoryMetaRow,
  MemoryStatusBadge,
} from "@/features/memory/memory-ui";
import { UiIconButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
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
  const [items, set_items] = useState<MemoryItem[]>([]);
  const [stats, set_stats] = useState<MemoryStats | null>(null);
  const [selected_item_id, set_selected_item_id] = useState("");
  const [status_filter, set_status_filter] = useState("");
  const [layer_filter, set_layer_filter] = useState<MemoryLayerFilter>("all");
  const [query, set_query] = useState("");
  const [loading, set_loading] = useState(false);
  const [cleaning, set_cleaning] = useState(false);
  const [deleting_item_id, set_deleting_item_id] = useState("");
  const [error, set_error] = useState<string | null>(null);

  const visible_items = useMemo(() => {
    return items.filter((item) => {
      if (status_filter && item.status !== status_filter) {
        return false;
      }
      return layer_filter === "all" || memory_layer_key(item.scope) === layer_filter;
    });
  }, [items, layer_filter, status_filter]);

  const selected_item = useMemo(
    () => visible_items.find((item) => item.entry_id === selected_item_id) ?? visible_items[0] ?? null,
    [selected_item_id, visible_items],
  );

  const load_memory = useCallback(async () => {
    set_loading(true);
    set_error(null);
    try {
      const [next_items, next_stats] = await Promise.all([
        query.trim()
          ? search_memory_items_api(agent.agent_id, query.trim(), 80)
          : list_memory_items_api(agent.agent_id, {
              limit: 120,
              status: status_filter,
            }),
        get_memory_stats_api(agent.agent_id),
      ]);
      set_items(next_items);
      set_stats(next_stats);
      set_selected_item_id((current) => {
        if (current && next_items.some((item) => item.entry_id === current)) {
          return current;
        }
        return next_items[0]?.entry_id ?? "";
      });
    } catch (load_error) {
      set_error(load_error instanceof Error ? load_error.message : "加载记忆失败");
      set_items([]);
      set_stats(null);
      set_selected_item_id("");
    } finally {
      set_loading(false);
    }
  }, [agent.agent_id, query, status_filter]);

  useEffect(() => {
    void load_memory();
  }, [load_memory]);

  const handle_delete = useCallback(
    async (item: MemoryItem) => {
      if (!window.confirm("确定删除这条记忆？删除后不会参与召回。")) {
        return;
      }
      set_deleting_item_id(item.entry_id);
      set_error(null);
      try {
        await delete_memory_item_api(agent.agent_id, item.entry_id);
        set_items((current) => current.filter((candidate) => candidate.entry_id !== item.entry_id));
        set_selected_item_id("");
        await load_memory();
      } catch (delete_error) {
        set_error(delete_error instanceof Error ? delete_error.message : "删除记忆失败");
      } finally {
        set_deleting_item_id("");
      }
    },
    [agent.agent_id, load_memory],
  );

  const handle_cleanup = useCallback(async () => {
    if (!window.confirm("清理无有效条目关联的会话摘要和检查点？")) {
      return;
    }
    set_cleaning(true);
    set_error(null);
    try {
      await cleanup_memory_api(agent.agent_id);
      await load_memory();
    } catch (cleanup_error) {
      set_error(cleanup_error instanceof Error ? cleanup_error.message : "清理记忆失败");
    } finally {
      set_cleaning(false);
    }
  }, [agent.agent_id, load_memory]);

  const stat_items = useMemo(
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
      <div className="mx-auto grid h-full min-h-0 w-full max-w-[1120px] grid-cols-1 gap-3 lg:grid-cols-[360px_minmax(360px,1fr)] xl:grid-cols-[380px_minmax(440px,1fr)]">
        <section className="flex min-h-0 flex-col overflow-hidden border-b border-(--divider-subtle-color) pb-3 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-3">
          <div className="flex h-11 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3.5">
            <div className="flex min-w-0 items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Brain className="h-3.5 w-3.5" />
              </span>
              <span className="truncate text-sm font-semibold text-(--text-strong)">记忆</span>
              <span className="text-[11px] font-medium text-(--text-soft)">
                {visible_items.length}
              </span>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <UiIconButton
                aria-label="刷新记忆"
                onClick={() => void load_memory()}
                size="sm"
                type="button"
              >
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
              </UiIconButton>
              <UiIconButton
                aria-label="清理脏记忆"
                disabled={cleaning}
                onClick={() => void handle_cleanup()}
                size="sm"
                title="清理脏记忆"
                type="button"
              >
                <Eraser className={cn("h-3.5 w-3.5", cleaning && "animate-pulse")} />
              </UiIconButton>
            </div>
          </div>

          <div className="grid grid-cols-4 gap-2 border-b border-(--divider-subtle-color) px-3.5 py-3">
            {stat_items.map((stat) => (
              <div className="min-w-0" key={stat.label}>
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
              control_size="sm"
              on_change={set_query}
              placeholder="搜索记忆"
              value={query}
            />
            <div className="grid grid-cols-2 gap-2">
              <UiSelectMenu
                aria_label="筛选记忆状态"
                on_change={set_status_filter}
                options={STATUS_OPTIONS}
                size="sm"
                value={status_filter}
              />
              <UiSelectMenu
                aria_label="筛选记忆层级"
                on_change={(value) => set_layer_filter(value as MemoryLayerFilter)}
                options={LAYER_OPTIONS}
                size="sm"
                value={layer_filter}
              />
            </div>
          </div>

          <MemoryItemList
            error={error}
            is_loading={loading}
            items={visible_items}
            on_select={set_selected_item_id}
            selected_item_id={selected_item?.entry_id ?? ""}
          />
        </section>

        <MemoryItemInspector
          is_deleting={selected_item ? deleting_item_id === selected_item.entry_id : false}
          item={selected_item}
          on_delete={handle_delete}
        />
      </div>
    </div>
  );
}

function MemoryItemList({
  error,
  is_loading,
  items,
  on_select,
  selected_item_id,
}: {
  error: string | null;
  is_loading: boolean;
  items: MemoryItem[];
  on_select: (entry_id: string) => void;
  selected_item_id: string;
}) {
  if (is_loading && items.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-soft)">
        <Loader2 className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (error) {
    return (
      <UiStateBlock description={error} size="sm" title="记忆加载失败" tone="danger" />
    );
  }
  if (items.length === 0) {
    return (
      <UiStateBlock description="当前筛选条件下没有记忆条目。" size="sm" title="暂无记忆" />
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      {items.map((item) => {
        const active = item.entry_id === selected_item_id;
        return (
          <UiListRow
            active={active}
            class_name="min-h-0 rounded-none border-b border-(--divider-subtle-color) px-3.5 py-3"
            key={item.entry_id}
            on_click={() => on_select(item.entry_id)}
          >
            <div className="flex min-w-0 items-center gap-2">
              <span className="min-w-0 flex-1 truncate text-[13px] font-semibold leading-5 text-(--text-strong)">
                {item.title || item.entry_id}
              </span>
              <MemoryStatusBadge status={item.status} />
            </div>
            <div className="mt-1 flex min-w-0 items-center gap-1.5 text-[11px] font-medium leading-4 text-(--text-soft)">
              <span>{memory_layer_label(item.scope)}</span>
              {item.score !== undefined ? <span>{format_memory_score(item.score)}</span> : null}
              <span>access {item.access_count}</span>
              {item.created_at ? <span>{format_memory_time(item.created_at)}</span> : null}
            </div>
            <p className="mt-1.5 line-clamp-2 whitespace-pre-wrap text-xs leading-5 text-(--text-default)">
              {item.content}
            </p>
          </UiListRow>
        );
      })}
    </div>
  );
}

function MemoryItemInspector({
  is_deleting,
  item,
  on_delete,
}: {
  is_deleting: boolean;
  item: MemoryItem | null;
  on_delete: (item: MemoryItem) => void;
}) {
  if (!item) {
    return (
      <section className="flex min-h-0 items-center justify-center px-6 text-xs text-(--text-soft)">
        未选择记忆
      </section>
    );
  }

  const raw_fields = (item.fields ?? []).filter((field) => field.value.trim() !== "");

  return (
    <section className="flex min-h-0 flex-col overflow-hidden">
      <div className="flex min-h-11 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4">
        <div className="flex min-w-0 items-center gap-2">
          <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
            <FileText className="h-3.5 w-3.5" />
          </span>
          <span className="truncate text-sm font-semibold leading-5 text-(--text-strong)">
            {item.title || item.entry_id}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <MemoryStatusBadge status={item.status} />
          <UiIconButton
            aria-label="删除记忆"
            disabled={is_deleting}
            onClick={() => on_delete(item)}
            size="sm"
            title="删除记忆"
            tone="danger"
            type="button"
          >
            {is_deleting ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Trash2 className="h-3.5 w-3.5" />
            )}
          </UiIconButton>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        <div className="flex flex-wrap items-center gap-2 text-[11px] font-medium leading-4 text-(--text-soft)">
          <span className="inline-flex items-center gap-1 rounded-md border border-(--divider-subtle-color) px-2 py-1">
            <Database className="h-3 w-3" />
            {memory_layer_label(item.scope)}
          </span>
          {item.kind ? <span>{item.kind}</span> : null}
          {item.category ? <span>{item.category}</span> : null}
          {item.priority ? <span>{item.priority}</span> : null}
          {item.created_at ? (
            <span className="inline-flex items-center gap-1">
              <Clock3 className="h-3 w-3" />
              {format_memory_time(item.created_at)}
            </span>
          ) : null}
          <span>access {item.access_count}</span>
          {item.score !== undefined ? <span>{format_memory_score(item.score)}</span> : null}
        </div>

        <div className="mt-4 border-y border-(--divider-subtle-color) py-3">
          <div className="mb-1 text-[11px] font-medium leading-4 text-(--text-soft)">内容</div>
          <p className="whitespace-pre-wrap text-[13px] leading-6 text-(--text-default)">
            {item.content}
          </p>
        </div>

        <dl className="mt-4 grid gap-1.5 text-[11px] leading-5">
          <MemoryMetaRow label="scope" value={item.scope} />
          <MemoryMetaRow label="source" value={item.source} />
          <MemoryMetaRow label="path" value={item.path} />
          <MemoryMetaRow label="session" value={item.session_key} />
          <MemoryMetaRow label="round" value={item.round_id} />
        </dl>

        {raw_fields.length > 0 ? (
          <details className="mt-4 border-t border-(--divider-subtle-color) pt-3 text-[11px] leading-5 text-(--text-soft)">
            <summary className="cursor-pointer select-none font-medium">
              原始字段 {raw_fields.length}
            </summary>
            <dl className="mt-2 grid gap-1.5">
              {raw_fields.map((field) => (
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
