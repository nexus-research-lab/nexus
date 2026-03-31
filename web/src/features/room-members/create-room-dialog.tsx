/**
 * 创建 Room 弹窗
 *
 * 复用 modal-dialog-surface 设计系统，与 AgentOptions / SkillDetailDialog 风格统一。
 * 使用 createPortal 渲染到 document.body，确保全页面居中显示。
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Bot, Check, Hash, Plus, Search, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";

interface CreateRoomDialogProps {
  agents: Agent[];
  is_open: boolean;
  is_creating?: boolean;
  on_cancel: () => void;
  on_confirm: (agent_ids: string[], name: string) => void;
}

const MAX_MEMBERS = 10;

export function CreateRoomDialog({
  agents,
  is_open,
  is_creating = false,
  on_cancel,
  on_confirm,
}: CreateRoomDialogProps) {
  const [search_query, set_search_query] = useState("");
  const [selected_ids, set_selected_ids] = useState<string[]>([]);
  const [room_name, set_room_name] = useState("");

  // 打开时重置状态
  useEffect(() => {
    if (is_open) {
      set_search_query("");
      set_selected_ids([]);
      set_room_name("");
    }
  }, [is_open]);

  // ESC 关闭
  useEffect(() => {
    if (!is_open) return;
    const handle = (e: KeyboardEvent) => {
      if (e.key === "Escape") on_cancel();
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, [is_open, on_cancel]);

  // 搜索过滤
  const filtered_agents = useMemo(() => {
    if (!search_query.trim()) return agents;
    const q = search_query.toLowerCase();
    return agents.filter((a) => a.name.toLowerCase().includes(q));
  }, [agents, search_query]);

  // 已选中的 Agent 对象列表
  const selected_agents = useMemo(
    () => agents.filter((a) => selected_ids.includes(a.agent_id)),
    [agents, selected_ids],
  );

  const toggle_agent = useCallback((agent_id: string) => {
    set_selected_ids((prev) => {
      if (prev.includes(agent_id)) {
        return prev.filter((id) => id !== agent_id);
      }
      if (prev.length >= MAX_MEMBERS) return prev;
      return [...prev, agent_id];
    });
  }, []);

  const handle_create = useCallback(() => {
    if (selected_ids.length === 0 || !room_name.trim()) return;
    on_confirm(selected_ids, room_name.trim());
  }, [selected_ids, room_name, on_confirm]);

  if (!is_open) return null;

  const can_create = selected_ids.length > 0 && room_name.trim().length > 0 && !is_creating;

  // Portal 渲染到 body，确保弹窗不受侧边栏 overflow 限制
  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={on_cancel}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="modal-dialog-surface radius-shell-xl flex w-full max-w-2xl flex-col overflow-hidden animate-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
        style={{ maxHeight: "80vh" }}
      >
        {/* 头部 — 与 AgentOptions / SkillDetailDialog 一致 */}
        <div className="flex items-center justify-between border-b modal-divider px-6 py-5">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl modal-card text-primary">
              <Hash className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-lg font-semibold tracking-tight text-slate-800">
                创建 Room
              </h2>
              <p className="truncate text-xs text-slate-500">
                设置名称并添加至少一个 Agent 成员
              </p>
            </div>
          </div>
          <button
            aria-label="关闭对话框"
            className="modal-btn-secondary rounded-xl p-2 text-slate-400 transition-colors hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_cancel}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* 内容区：左右两栏 */}
        <div className="soft-scrollbar flex flex-1 gap-5 overflow-y-auto px-6 py-5">
          {/* 左栏：Agent 列表 */}
          <div className="flex min-w-0 flex-1 flex-col gap-3">
            {/* 搜索框 */}
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                className="modal-card w-full rounded-xl py-2 pl-8 pr-3 text-sm text-slate-800 placeholder:text-slate-400 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
                onChange={(e) => set_search_query(e.target.value)}
                placeholder="搜索 Agent..."
                type="text"
                value={search_query}
              />
            </div>

            {/* Agent 计数 */}
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-400">
              All Agents ({filtered_agents.length})
            </p>

            {/* Agent 列表 */}
            <div className="flex flex-col gap-1.5 overflow-y-auto pr-1" style={{ maxHeight: 280 }}>
              {filtered_agents.map((agent) => {
                const is_selected = selected_ids.includes(agent.agent_id);
                return (
                  <button
                    key={agent.agent_id}
                    className={cn(
                      "modal-card flex w-full items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-all duration-200",
                      is_selected && "modal-card-active ring-1 ring-primary/30",
                    )}
                    onClick={() => toggle_agent(agent.agent_id)}
                    type="button"
                  >
                    {/* Agent 头像 */}
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-slate-100/80 text-[11px] font-bold text-slate-600">
                      <Bot className="h-4 w-4" />
                    </div>

                    {/* Agent 信息 */}
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold text-slate-950/88">
                        {agent.name}
                      </p>
                      <p className="truncate text-[11px] text-slate-500">
                        {agent.options?.system_prompt
                          ? agent.options.system_prompt.slice(0, 50) + (agent.options.system_prompt.length > 50 ? "..." : "")
                          : agent.status ?? "idle"}
                      </p>
                    </div>

                    {/* 已选标记 */}
                    <div
                      className={cn(
                        "flex h-6 w-6 shrink-0 items-center justify-center rounded-full transition-all",
                        is_selected
                          ? "bg-primary text-white"
                          : "border border-slate-300/60 text-slate-400",
                      )}
                    >
                      {is_selected ? (
                        <Check className="h-3 w-3" />
                      ) : (
                        <Plus className="h-3 w-3" />
                      )}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {/* 右栏：已选成员 */}
          <div className="flex w-[220px] shrink-0 flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-400">
              已选成员 ({selected_ids.length} / {MAX_MEMBERS})
            </p>

            <div className="modal-card flex flex-1 flex-col gap-1 overflow-y-auto rounded-2xl p-2.5">
              {selected_agents.length > 0 ? (
                selected_agents.map((agent) => (
                  <div
                    key={agent.agent_id}
                    className="flex items-center gap-2 rounded-xl px-2.5 py-1.5 transition-colors hover:bg-black/3"
                  >
                    <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-slate-100/80 text-slate-600">
                      <Bot className="h-3 w-3" />
                    </div>
                    <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-slate-800">
                      {agent.name}
                    </span>
                    <button
                      className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-slate-400 transition-colors hover:text-red-500"
                      onClick={() => toggle_agent(agent.agent_id)}
                      type="button"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                ))
              ) : (
                <p className="flex flex-1 items-center justify-center text-[12px] text-slate-400">
                  从左侧添加 Agent 成员
                </p>
              )}
            </div>
          </div>
        </div>

        {/* 底部栏 — 与 AgentOptions footer 一致 */}
        <div className="flex items-center justify-between gap-4 border-t modal-divider px-6 py-5">
          {/* Room 名称输入 */}
          <div className="flex items-center gap-2.5">
            <label className="shrink-0 text-[13px] font-semibold text-slate-700">
              Room 名称
            </label>
            <input
              className="modal-card w-48 rounded-xl px-3 py-2 text-sm text-slate-800 placeholder:text-slate-400 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
              maxLength={64}
              onChange={(e) => set_room_name(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && can_create) handle_create(); }}
              placeholder="e.g. dev-team"
              type="text"
              value={room_name}
            />
          </div>

          {/* 操作按钮 */}
          <div className="flex items-center gap-3">
            <button
              className="modal-btn-secondary rounded-xl px-4 py-2 text-[13px] font-semibold transition-colors"
              onClick={on_cancel}
              type="button"
            >
              取消
            </button>
            <button
              className={cn(
                "rounded-xl px-5 py-2 text-[13px] font-semibold text-white transition-all",
                can_create
                  ? "bg-primary shadow-[0_8px_20px_rgba(var(--color-primary-rgb,99,102,241),0.25)] hover:brightness-110"
                  : "modal-card cursor-not-allowed text-slate-400",
              )}
              disabled={!can_create}
              onClick={handle_create}
              type="button"
            >
              {is_creating ? "创建中..." : "创建 Room"}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
