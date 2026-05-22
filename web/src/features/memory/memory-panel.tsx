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
import { format_memory_time } from "@/features/memory/memory-utils";
import { MemoryStatusBadge } from "@/features/memory/memory-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiInput, UiSearchInput, UiTextarea } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  CapabilityFilterBar,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
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

export function MemoryPanel() {
  const { t } = useI18n();
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
          badge={t("capability.memory_badge", { count: stats?.total ?? items.length })}
          density="compact"
          leading={<Database className="h-4 w-4" />}
          subtitle={selected_agent ? selected_agent.workspace_path : t("capability.memory_subtitle")}
          title={t("capability.memory")}
          trailing={
            <>
              <UiSelectMenu
                aria_label="选择记忆 Agent"
                class_name="min-w-[160px]"
                on_change={set_agent_id}
                options={agents.map((agent) => ({
                  value: agent.agent_id,
                  label: agent.name,
                }))}
                size="sm"
                value={agent_id}
              />
              <WorkspaceSurfaceToolbarAction disabled={loading || !agent_id} onClick={refresh}>
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction disabled={cleaning || !agent_id} onClick={handle_cleanup}>
                <Eraser className={cn("h-3.5 w-3.5", cleaning && "animate-pulse")} />
                清理
              </WorkspaceSurfaceToolbarAction>
            </>
          }
        />
      }
      stable_gutter
    >
      <CapabilityPageLayout
        description={t("capability.memory_intro_description")}
        title={t("capability.memory_intro_title")}
      >
        <CapabilitySectionHeader title={t("capability.memory_overview_title")} />
        <section className="mb-5 grid gap-3 sm:grid-cols-4">
          {stat_items.map(([label, value]) => (
            <UiPanel
              class_name="min-w-0"
              key={label}
              padding="sm"
              variant="inset"
            >
              <div className="text-[11px] font-medium text-(--text-soft)">{label}</div>
              <div className="mt-1 text-base font-semibold tabular-nums text-(--text-strong)">{value}</div>
            </UiPanel>
          ))}
        </section>

        <CapabilityFilterBar>
          <UiSearchInput
            class_name="h-10 min-w-0 flex-1 rounded-[13px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3.5"
            input_class_name="text-[14px]"
            on_change={set_query}
            placeholder={t("capability.memory_search_placeholder")}
            value={query}
          />
          <UiSelectMenu
            aria_label={t("capability.memory_filter_status_aria")}
            class_name="shrink-0 sm:w-[184px]"
            on_change={set_status}
            options={STATUS_OPTIONS}
            size="sm"
            value={status}
          />
          <UiButton
            class_name="shrink-0"
            disabled={loading || !agent_id}
            onClick={refresh}
            tone="primary"
            type="button"
            variant="solid"
          >
            <Search className="h-3.5 w-3.5" />
            查询
          </UiButton>
        </CapabilityFilterBar>

        <UiPanel padding="sm" variant="inset">
          <div className="grid gap-2 md:grid-cols-[220px_1fr_auto]">
            <UiInput
              onChange={(event) => set_new_title(event.target.value)}
              placeholder="标题"
              value={new_title}
              variant="surface"
            />
            <UiInput
              onChange={(event) => set_new_content(event.target.value)}
              placeholder="新增候选记忆"
              value={new_content}
              variant="surface"
            />
            <UiButton
              disabled={!new_content.trim() || loading}
              onClick={handle_add}
              type="button"
            >
              <Check className="h-3.5 w-3.5" />
              添加
            </UiButton>
          </div>
        </UiPanel>

        <UiPanel class_name="overflow-hidden" padding="none" variant="inset">
          {items.length === 0 ? (
            <UiStateBlock description="当前筛选条件下没有可管理的记忆条目。" size="sm" title="暂无记忆条目" />
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
                        <MemoryStatusBadge status={item.status} />
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
                        {item.created_at ? ` · ${format_memory_time(item.created_at)}` : ""}
                        {` · access ${item.access_count}`}
                      </div>
                      {is_editing ? (
                        <UiTextarea
                          class_name="mt-2"
                          control_size="md"
                          onChange={(event) => set_editing_content(event.target.value)}
                          value={editing_content}
                          variant="surface"
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
                          <UiIconButton
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "save")}
                            size="md"
                            title="保存"
                            type="button"
                          >
                            <Check className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            onClick={() => set_editing_id("")}
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
                            disabled={is_mutating}
                            onClick={() => {
                              set_editing_id(item.entry_id);
                              set_editing_content(item.content);
                            }}
                            size="md"
                            title="编辑"
                            type="button"
                          >
                            <Database className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "promote")}
                            size="md"
                            title="提升"
                            type="button"
                          >
                            <ShieldCheck className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "ignore")}
                            size="md"
                            title="忽略"
                            type="button"
                          >
                            <X className="h-3.5 w-3.5" />
                          </UiIconButton>
                          <UiIconButton
                            disabled={is_mutating}
                            onClick={() => void mutate_item(item, "delete")}
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
        </UiPanel>
      </CapabilityPageLayout>
      <FeedbackBannerStack
        items={feedback ? [
          {
            key: "memory-feedback",
            message: feedback.message,
            on_dismiss: () => set_feedback(null),
            title: feedback.tone === "error" ? "操作失败" : feedback.tone === "warning" ? "需要注意" : "操作完成",
            tone: feedback.tone,
          },
        ] : []}
      />
    </WorkspaceSurfaceScaffold>
  );
}
