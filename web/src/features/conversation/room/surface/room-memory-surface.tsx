"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Brain, Check, Pencil, Plus, RefreshCw, Trash2, X } from "lucide-react";

import {
  add_room_shared_memory_item_api,
  delete_room_shared_memory_item_api,
  get_room_shared_memory_stats_api,
  list_room_shared_memory_items_api,
  update_room_shared_memory_item_api,
} from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import {
  format_memory_time,
  memory_scope_label,
} from "@/features/memory/memory-utils";
import { MemoryMetaChip, MemoryStatusBadge } from "@/features/memory/memory-ui";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { MemoryItem, MemoryStats } from "@/types/memory/memory";

interface RoomMemorySurfaceProps {
  room_id: string | null;
  conversation_id: string | null;
  header_action?: ReactNode;
}

export function RoomMemorySurface({
  room_id,
  conversation_id,
  header_action,
}: RoomMemorySurfaceProps) {
  const [items, set_items] = useState<MemoryItem[]>([]);
  const [stats, set_stats] = useState<MemoryStats | null>(null);
  const [loading, set_loading] = useState(false);
  const [mutating_id, set_mutating_id] = useState("");
  const [error, set_error] = useState<string | null>(null);
  const [new_title, set_new_title] = useState("");
  const [new_content, set_new_content] = useState("");
  const [editing_id, set_editing_id] = useState("");
  const [editing_title, set_editing_title] = useState("");
  const [editing_content, set_editing_content] = useState("");

  const can_load = Boolean(room_id && conversation_id);

  const refresh = useCallback(async () => {
    if (!room_id || !conversation_id) {
      set_items([]);
      set_stats(null);
      return;
    }
    set_loading(true);
    set_error(null);
    try {
      const [next_items, next_stats] = await Promise.all([
        list_room_shared_memory_items_api(room_id, conversation_id, { limit: 200 }),
        get_room_shared_memory_stats_api(room_id, conversation_id),
      ]);
      set_items(next_items);
      set_stats(next_stats);
    } catch (err) {
      set_error(err instanceof Error ? err.message : "读取 Room 记忆失败");
    } finally {
      set_loading(false);
    }
  }, [conversation_id, room_id]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const stat_items = useMemo<Array<[string, number]>>(() => [
    ["总数", stats?.total ?? 0],
    ["候选", stats?.candidate ?? 0],
    ["已访问", stats?.accessed ?? 0],
    ["检查点", stats?.checkpointed ?? 0],
  ], [stats]);

  const handle_add = async () => {
    if (!room_id || !conversation_id || !new_content.trim()) {
      return;
    }
    set_loading(true);
    set_error(null);
    try {
      await add_room_shared_memory_item_api(room_id, conversation_id, {
        title: new_title.trim(),
        content: new_content.trim(),
        kind: "LRN",
        category: "room",
        status: "candidate",
        priority: "medium",
        source: "room_manual",
      });
      set_new_title("");
      set_new_content("");
      await refresh();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "新增 Room 记忆失败");
    } finally {
      set_loading(false);
    }
  };

  const begin_edit = (item: MemoryItem) => {
    set_editing_id(item.entry_id);
    set_editing_title(item.title);
    set_editing_content(item.content);
  };

  const cancel_edit = () => {
    set_editing_id("");
    set_editing_title("");
    set_editing_content("");
  };

  const handle_save = async (item: MemoryItem) => {
    if (!room_id || !conversation_id || !editing_content.trim()) {
      return;
    }
    set_mutating_id(item.entry_id);
    set_error(null);
    try {
      await update_room_shared_memory_item_api(room_id, conversation_id, item.entry_id, {
        title: editing_title.trim(),
        content: editing_content.trim(),
      });
      cancel_edit();
      await refresh();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "保存 Room 记忆失败");
    } finally {
      set_mutating_id("");
    }
  };

  const handle_delete = async (item: MemoryItem) => {
    if (!room_id || !conversation_id) {
      return;
    }
    if (!window.confirm("确定删除这条 Room 记忆？")) {
      return;
    }
    set_mutating_id(item.entry_id);
    set_error(null);
    try {
      await delete_room_shared_memory_item_api(room_id, conversation_id, item.entry_id);
      if (editing_id === item.entry_id) {
        cancel_edit();
      }
      await refresh();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "删除 Room 记忆失败");
    } finally {
      set_mutating_id("");
    }
  };

  return (
    <WorkspaceSurfaceScaffold
      body_scrollable
      header={(
        <WorkspaceSurfaceHeader
          badge={`${stats?.total ?? items.length}`}
          density="compact"
          leading={<Brain className="h-4 w-4" />}
          subtitle="当前 conversation 的 room_shared 记忆"
          title="Room Memory"
          trailing={(
            <>
              <WorkspaceSurfaceToolbarAction disabled={loading || !can_load} onClick={refresh}>
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
                刷新
              </WorkspaceSurfaceToolbarAction>
              {header_action}
            </>
          )}
        />
      )}
      stable_gutter
    >
      <div className="soft-scrollbar h-full min-h-0 overflow-y-auto px-4 py-4">
        <section className="mb-4 grid grid-cols-2 gap-2">
          {stat_items.map(([label, value]) => (
            <div
              className="min-w-0 rounded-[8px] border border-(--divider-subtle-color) px-3 py-2"
              key={label}
            >
              <div className="text-[11px] font-medium text-(--text-soft)">{label}</div>
              <div className="mt-1 text-base font-semibold tabular-nums text-(--text-strong)">{value}</div>
            </div>
          ))}
        </section>

        <section className="mb-4 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-3">
          <div className="grid gap-2">
            <UiInput
              disabled={!can_load || loading}
              onChange={(event) => set_new_title(event.target.value)}
              placeholder="标题"
              value={new_title}
              variant="surface"
            />
            <UiTextarea
              class_name="min-h-[76px]"
              disabled={!can_load || loading}
              onChange={(event) => set_new_content(event.target.value)}
              placeholder="写入 Room shared memory"
              value={new_content}
              variant="surface"
            />
            <div className="flex justify-end">
              <UiButton
                disabled={!can_load || loading || !new_content.trim()}
                onClick={handle_add}
                size="sm"
              >
                <Plus className="h-3.5 w-3.5" />
                添加
              </UiButton>
            </div>
          </div>
        </section>

        {!can_load ? (
          <UiStateBlock
            description="先创建或选择一个 Room conversation。"
            title="暂无可读取的 Room 记忆"
          />
        ) : error ? (
          <UiStateBlock
            description={error}
            title="读取失败"
            tone="danger"
          />
        ) : loading && items.length === 0 ? (
          <UiStateBlock
            description="正在读取当前 Room conversation 的共享记忆。"
            title="加载中"
          />
        ) : items.length === 0 ? (
          <UiStateBlock
            description="当前 conversation 还没有自动提取或手动写入的 shared memory。"
            title="还没有 Room 记忆"
          />
        ) : (
          <div className="space-y-2">
            {items.map((item) => (
              <article
                className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-3"
                key={item.entry_id}
              >
                <div className="flex min-w-0 items-start justify-between gap-3">
                  <div className="min-w-0">
                    {editing_id === item.entry_id ? (
                      <UiInput
                        onChange={(event) => set_editing_title(event.target.value)}
                        value={editing_title}
                        variant="surface"
                      />
                    ) : (
                      <h3 className="truncate text-[13px] font-semibold text-(--text-strong)">
                        {item.title || item.entry_id}
                      </h3>
                    )}
                    <div className="mt-1 flex flex-wrap items-center gap-1.5">
                      <MemoryStatusBadge status={item.status} />
                      <MemoryMetaChip>{item.kind}</MemoryMetaChip>
                      {item.priority ? <MemoryMetaChip>{item.priority}</MemoryMetaChip> : null}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <span className="hidden text-[10px] tabular-nums text-(--text-muted) sm:inline">
                      {format_memory_time(item.created_at)}
                    </span>
                    {editing_id === item.entry_id ? (
                      <>
                        <UiIconButton
                          aria-label="保存"
                          disabled={mutating_id === item.entry_id || !editing_content.trim()}
                          onClick={() => handle_save(item)}
                          size="xs"
                          title="保存"
                        >
                          <Check className="h-3.5 w-3.5" />
                        </UiIconButton>
                        <UiIconButton
                          aria-label="取消"
                          disabled={mutating_id === item.entry_id}
                          onClick={cancel_edit}
                          size="xs"
                          title="取消"
                        >
                          <X className="h-3.5 w-3.5" />
                        </UiIconButton>
                      </>
                    ) : (
                      <>
                        <UiIconButton
                          aria-label="编辑"
                          disabled={mutating_id === item.entry_id}
                          onClick={() => begin_edit(item)}
                          size="xs"
                          title="编辑"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </UiIconButton>
                        <UiIconButton
                          aria-label="删除"
                          disabled={mutating_id === item.entry_id}
                          onClick={() => handle_delete(item)}
                          size="xs"
                          title="删除"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </UiIconButton>
                      </>
                    )}
                  </div>
                </div>
                {editing_id === item.entry_id ? (
                  <UiTextarea
                    class_name="mt-2 min-h-[90px]"
                    onChange={(event) => set_editing_content(event.target.value)}
                    value={editing_content}
                    variant="surface"
                  />
                ) : (
                  <p className="mt-2 line-clamp-4 text-[12px] leading-5 text-(--text-default)">
                    {item.content}
                  </p>
                )}
                <div className="mt-2 flex flex-wrap gap-1.5">
                  <MemoryMetaChip>{memory_scope_label(item.scope)}</MemoryMetaChip>
                  <MemoryMetaChip>access {item.access_count}</MemoryMetaChip>
                  {item.source ? <MemoryMetaChip>{item.source}</MemoryMetaChip> : null}
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </WorkspaceSurfaceScaffold>
  );
}
