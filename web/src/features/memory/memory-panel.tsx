"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Check,
  Database,
  Eraser,
  RefreshCw,
  Search,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";

import { get_agents } from "@/lib/api/agent-manage-api";
import {
  add_memory_item_api,
  cleanup_memory_api,
  delete_memory_item_api,
  get_memory_stats_api,
  ignore_memory_item_api,
  list_memory_items_api,
  promote_memory_item_api,
  search_memory_items_api,
  update_memory_item_api,
} from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";
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

function format_time(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function MemoryPanel() {
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_id, set_agent_id] = useState("");
  const [items, set_items] = useState<MemoryItem[]>([]);
  const [stats, set_stats] = useState<MemoryStats | null>(null);
  const [status, set_status] = useState("");
  const [query, set_query] = useState("");
  const [new_title, set_new_title] = useState("");
  const [new_content, set_new_content] = useState("");
  const [editing_id, set_editing_id] = useState("");
  const [editing_content, set_editing_content] = useState("");
  const [loading, set_loading] = useState(false);
  const [cleaning, set_cleaning] = useState(false);
  const [mutating_id, set_mutating_id] = useState("");
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);

  const selected_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === agent_id) ?? null,
    [agent_id, agents],
  );

  const refresh = useCallback(async () => {
    if (!agent_id) {
      return;
    }
    set_loading(true);
    try {
      const [next_items, next_stats] = await Promise.all([
        query.trim()
          ? search_memory_items_api(agent_id, query.trim(), 100)
          : list_memory_items_api(agent_id, { limit: 200, status }),
        get_memory_stats_api(agent_id),
      ]);
      set_items(next_items);
      set_stats(next_stats);
    } catch (error) {
      set_feedback({
        tone: "error",
        message: error instanceof Error ? error.message : "刷新记忆失败",
      });
    } finally {
      set_loading(false);
    }
  }, [agent_id, query, status]);

  useEffect(() => {
    let ignore = false;
    void get_agents()
      .then((next_agents) => {
        if (ignore) {
          return;
        }
        set_agents(next_agents);
        set_agent_id((current) => current || next_agents[0]?.agent_id || "");
      })
      .catch((error) => {
        if (!ignore) {
          set_feedback({
            tone: "error",
            message: error instanceof Error ? error.message : "加载 Agent 失败",
          });
        }
      });
    return () => {
      ignore = true;
    };
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handle_add = async () => {
    if (!agent_id || !new_content.trim()) {
      return;
    }
    set_loading(true);
    try {
      await add_memory_item_api(agent_id, {
        title: new_title.trim(),
        content: new_content.trim(),
        kind: "LRN",
        category: "preference",
        status: "candidate",
        priority: "medium",
        source: "manual",
      });
      set_new_title("");
      set_new_content("");
      set_feedback({ tone: "success", message: "记忆已加入候选区" });
      await refresh();
    } catch (error) {
      set_feedback({
        tone: "error",
        message: error instanceof Error ? error.message : "新增记忆失败",
      });
    } finally {
      set_loading(false);
    }
  };

  const mutate_item = async (
    item: MemoryItem,
    action: "promote" | "ignore" | "delete" | "save",
  ) => {
    if (!agent_id) {
      return;
    }
    if (action === "delete" && !window.confirm("确定删除这条记忆？删除后不会参与召回。")) {
      return;
    }
    set_mutating_id(item.entry_id);
    try {
      if (action === "promote") {
        await promote_memory_item_api(agent_id, item.entry_id, "memory");
        set_feedback({ tone: "success", message: "记忆已提升到 MEMORY.md" });
      } else if (action === "ignore") {
        await ignore_memory_item_api(agent_id, item.entry_id);
        set_feedback({ tone: "success", message: "候选记忆已忽略" });
      } else if (action === "delete") {
        await delete_memory_item_api(agent_id, item.entry_id);
        set_feedback({ tone: "success", message: "记忆已删除" });
      } else {
        await update_memory_item_api(agent_id, item.entry_id, {
          content: editing_content,
        });
        set_editing_id("");
        set_editing_content("");
        set_feedback({ tone: "success", message: "记忆已保存" });
      }
      await refresh();
    } catch (error) {
      set_feedback({
        tone: "error",
        message: error instanceof Error ? error.message : "操作失败",
      });
    } finally {
      set_mutating_id("");
    }
  };

  const handle_cleanup = async () => {
    if (!agent_id || !window.confirm("清理无有效条目关联的会话摘要和检查点？")) {
      return;
    }
    set_cleaning(true);
    try {
      const result = await cleanup_memory_api(agent_id);
      set_feedback({
        tone: "success",
        message: `已清理 ${result.removed_session_files + result.removed_checkpoints + result.removed_empty_diaries} 项脏数据`,
      });
      await refresh();
    } catch (error) {
      set_feedback({
        tone: "error",
        message: error instanceof Error ? error.message : "清理记忆失败",
      });
    } finally {
      set_cleaning(false);
    }
  };

  const stat_items: Array<[string, number]> = [
    ["总数", stats?.total ?? 0],
    ["候选", stats?.candidate ?? 0],
    ["自动", stats?.by_status?.auto ?? 0],
    ["已提升", stats?.by_status?.promoted ?? 0],
  ];

  return (
    <WorkspaceSurfaceScaffold
      body_scrollable
      header={
        <WorkspaceSurfaceHeader
          density="compact"
          leading={<Database className="h-4 w-4" />}
          subtitle={selected_agent ? selected_agent.workspace_path : "选择一个 Agent 管理本地记忆"}
          title="Memory"
          trailing={
            <>
              <select
                className="h-8 min-w-[160px] rounded-md border border-(--divider-subtle-color) bg-transparent px-2 text-[12px] text-(--text-strong)"
                onChange={(event) => set_agent_id(event.target.value)}
                value={agent_id}
              >
                {agents.map((agent) => (
                  <option key={agent.agent_id} value={agent.agent_id}>
                    {agent.name}
                  </option>
                ))}
              </select>
              <WorkspaceSurfaceToolbarAction disabled={loading || !agent_id} onClick={refresh}>
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
                刷新
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction disabled={cleaning || !agent_id} onClick={handle_cleanup}>
                <Eraser className={cn("h-3.5 w-3.5", cleaning && "animate-pulse")} />
                清理
              </WorkspaceSurfaceToolbarAction>
            </>
          }
        />
      }
      body_class_name="px-5 py-4 xl:px-6"
    >
      <div className="mx-auto flex max-w-6xl flex-col gap-4">
        {feedback ? (
          <div
            className={cn(
              "flex items-center justify-between rounded-md border px-3 py-2 text-[12px]",
              feedback.tone === "error"
                ? "border-red-500/30 bg-red-500/10 text-red-700"
                : feedback.tone === "warning"
                  ? "border-amber-500/30 bg-amber-500/10 text-amber-700"
                  : "border-emerald-500/30 bg-emerald-500/10 text-emerald-700",
            )}
          >
            <span>{feedback.message}</span>
            <button onClick={() => set_feedback(null)} type="button">
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ) : null}

        <section className="grid gap-3 sm:grid-cols-4">
          {stat_items.map(([label, value]) => (
            <div
              className="rounded-md border border-(--divider-subtle-color) bg-(--surface-background) px-4 py-3"
              key={label}
            >
              <div className="text-[11px] font-medium text-(--text-soft)">{label}</div>
              <div className="mt-1 text-base font-semibold tabular-nums text-(--text-strong)">{value}</div>
            </div>
          ))}
        </section>

        <section className="rounded-md border border-(--divider-subtle-color) bg-(--surface-background) p-3">
          <div className="grid gap-2 md:grid-cols-[180px_1fr_auto]">
            <select
              className="h-9 rounded-md border border-(--divider-subtle-color) bg-transparent px-2 text-[12px]"
              onChange={(event) => set_status(event.target.value)}
              value={status}
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <label className="flex h-9 items-center gap-2 rounded-md border border-(--divider-subtle-color) px-3">
              <Search className="h-3.5 w-3.5 text-(--text-soft)" />
              <input
                className="min-w-0 flex-1 bg-transparent text-[12px] outline-none"
                onChange={(event) => set_query(event.target.value)}
                placeholder="搜索关键词"
                value={query}
              />
            </label>
            <button
              className="inline-flex h-9 items-center justify-center gap-1.5 rounded-md bg-(--primary) px-3 text-[12px] font-semibold text-white disabled:opacity-50"
              disabled={loading || !agent_id}
              onClick={refresh}
              type="button"
            >
              <Search className="h-3.5 w-3.5" />
              查询
            </button>
          </div>
        </section>

        <section className="rounded-md border border-(--divider-subtle-color) bg-(--surface-background) p-3">
          <div className="grid gap-2 md:grid-cols-[220px_1fr_auto]">
            <input
              className="h-9 rounded-md border border-(--divider-subtle-color) bg-transparent px-3 text-[12px] outline-none"
              onChange={(event) => set_new_title(event.target.value)}
              placeholder="标题"
              value={new_title}
            />
            <input
              className="h-9 rounded-md border border-(--divider-subtle-color) bg-transparent px-3 text-[12px] outline-none"
              onChange={(event) => set_new_content(event.target.value)}
              placeholder="新增候选记忆"
              value={new_content}
            />
            <button
              className="inline-flex h-9 items-center justify-center gap-1.5 rounded-md border border-(--divider-subtle-color) px-3 text-[12px] font-semibold disabled:opacity-50"
              disabled={!new_content.trim() || loading}
              onClick={handle_add}
              type="button"
            >
              <Check className="h-3.5 w-3.5" />
              添加
            </button>
          </div>
        </section>

        <section className="overflow-hidden rounded-md border border-(--divider-subtle-color) bg-(--surface-background)">
          {items.length === 0 ? (
            <div className="px-4 py-8 text-center text-[12px] text-(--text-soft)">
              暂无记忆条目
            </div>
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {items.map((item) => {
                const is_editing = editing_id === item.entry_id;
                const is_mutating = mutating_id === item.entry_id;
                return (
                  <article className="grid gap-3 px-4 py-3 md:grid-cols-[1fr_auto]" key={item.entry_id}>
                    <div className="min-w-0">
                      <div className="flex min-w-0 flex-wrap items-center gap-2">
                        <span className="truncate text-[13px] font-semibold leading-5 text-(--text-strong)">
                          {item.title || item.entry_id}
                        </span>
                        <span className="rounded-full border border-(--divider-subtle-color) px-2 py-0.5 text-[11px] font-medium leading-4 text-(--text-default)">
                          {item.status}
                        </span>
                        {item.priority ? (
                          <span className="text-[11px] text-(--text-soft)">{item.priority}</span>
                        ) : null}
                      </div>
                      <div className="mt-1 text-[11px] text-(--text-soft)">
                        {item.kind}
                        {item.category ? ` / ${item.category}` : ""}
                        {item.source ? ` · ${item.source}` : ""}
                        {item.path ? ` · ${item.path}` : ""}
                        {item.scope ? ` · ${item.scope}` : ""}
                        {item.session_key ? ` · ${item.session_key}` : ""}
                        {item.round_id ? ` · ${item.round_id}` : ""}
                        {item.created_at ? ` · ${format_time(item.created_at)}` : ""}
                        {` · access ${item.access_count}`}
                      </div>
                      {is_editing ? (
                        <textarea
                          className="mt-2 min-h-24 w-full resize-y rounded-md border border-(--divider-subtle-color) bg-transparent p-2 text-[12px] outline-none"
                          onChange={(event) => set_editing_content(event.target.value)}
                          value={editing_content}
                        />
                      ) : (
                        <p className="mt-2 line-clamp-3 whitespace-pre-wrap text-[12px] leading-5 text-(--text-default)">
                          {item.content}
                        </p>
                      )}
                    </div>
                    <div className="flex items-start gap-1">
                      {is_editing ? (
                        <>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color) disabled:opacity-50"
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "save")}
                            title="保存"
                            type="button"
                          >
                            <Check className="h-3.5 w-3.5" />
                          </button>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color)"
                            onClick={() => set_editing_id("")}
                            title="取消"
                            type="button"
                          >
                            <X className="h-3.5 w-3.5" />
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color) disabled:opacity-50"
                            disabled={is_mutating}
                            onClick={() => {
                              set_editing_id(item.entry_id);
                              set_editing_content(item.content);
                            }}
                            title="编辑"
                            type="button"
                          >
                            <Database className="h-3.5 w-3.5" />
                          </button>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color) disabled:opacity-50"
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "promote")}
                            title="提升"
                            type="button"
                          >
                            <ShieldCheck className="h-3.5 w-3.5" />
                          </button>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color) disabled:opacity-50"
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "ignore")}
                            title="忽略"
                            type="button"
                          >
                            <X className="h-3.5 w-3.5" />
                          </button>
                          <button
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-(--divider-subtle-color) text-red-600 disabled:opacity-50"
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "delete")}
                            title="删除"
                            type="button"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        </>
                      )}
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </section>
      </div>
    </WorkspaceSurfaceScaffold>
  );
}
