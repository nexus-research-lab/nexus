/**
 * 创建 Room 弹窗
 *
 * 复用 dialog-shell 设计系统，与 AgentOptions / SkillDetailDialog 风格统一。
 * 使用 createPortal 渲染到 document.body，确保全页面居中显示。
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Bot, Check, Hash, Plus, Search, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { get_icon_avatar_src, get_initials, get_room_avatar_icon_id } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_SHELL_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/workspace-catalog-card";
import { Agent } from "@/types/agent";

interface CreateRoomDialogProps {
  agents: Agent[];
  is_open: boolean;
  is_creating?: boolean;
  mode?: "create" | "manage";
  dialog_title?: string;
  dialog_subtitle?: string;
  confirm_label?: string;
  initial_name?: string;
  initial_avatar?: string;
  initial_selected_agent_ids?: string[];
  on_cancel: () => void;
  on_confirm: (agent_ids: string[], name: string, avatar?: string) => void;
}

const MAX_MEMBERS = 10;
const EMPTY_AGENT_IDS: string[] = [];

export function CreateRoomDialog({
  agents,
  is_open,
  is_creating = false,
  mode = "create",
  dialog_title,
  dialog_subtitle,
  confirm_label,
  initial_name = "",
  initial_avatar = "",
  initial_selected_agent_ids,
  on_cancel,
  on_confirm,
}: CreateRoomDialogProps) {
  const { t } = useI18n();
  const [search_query, set_search_query] = useState("");
  const [selected_ids, set_selected_ids] = useState<string[]>([]);
  const [room_name, set_room_name] = useState("");
  const [selected_avatar, set_selected_avatar] = useState("");
  const normalized_initial_selected_ids = initial_selected_agent_ids ?? EMPTY_AGENT_IDS;
  // 中文注释：数组 props 往往每次 render 都是新引用，依赖序列化后的稳定签名，
  // 避免弹窗打开时因默认空数组或父层重建数组而反复 setState。
  const initial_selected_ids_signature = useMemo(
    () => JSON.stringify(normalized_initial_selected_ids),
    [normalized_initial_selected_ids],
  );
  const stable_initial_selected_ids = useMemo(
    () => JSON.parse(initial_selected_ids_signature) as string[],
    [initial_selected_ids_signature],
  );

  // 打开时重置状态
  useEffect(() => {
    if (is_open) {
      set_search_query("");
      set_selected_ids(stable_initial_selected_ids);
      set_room_name(initial_name);
      set_selected_avatar(initial_avatar);
    }
  }, [initial_avatar, initial_name, initial_selected_ids_signature, is_open, stable_initial_selected_ids]);

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
    on_confirm(selected_ids, room_name.trim(), selected_avatar || undefined);
  }, [on_confirm, room_name, selected_avatar, selected_ids]);

  if (!is_open) return null;

  const can_create = selected_ids.length > 0 && room_name.trim().length > 0 && !is_creating;
  const preview_avatar_id = get_room_avatar_icon_id(null, room_name, selected_avatar);
  const preview_avatar_src = get_icon_avatar_src(preview_avatar_id);
  const resolved_dialog_title = dialog_title ?? (mode === "manage" ? t("room.manage_dialog_title") : t("room.create_dialog_title"));
  const resolved_dialog_subtitle = dialog_subtitle ?? (mode === "manage" ? t("room.manage_dialog_subtitle") : t("room.create_dialog_subtitle"));
  const resolved_confirm_label = confirm_label ?? (mode === "manage" ? t("common.save") : t("room.create_action"));

  if (typeof document === "undefined") {
    return null;
  }

  // Portal 渲染到 body，确保弹窗不受侧边栏 overflow 限制
  return createPortal(
    <>
      <div
        aria-hidden="true"
        className={cn(DIALOG_BACKDROP_CLASS_NAME, "z-[9998]")}
        data-modal-root="true"
        onClick={on_cancel}
      />
      <div
        data-modal-root="true"
        aria-modal="true"
        className="fixed inset-0 z-[9999] flex items-center justify-center p-6"
        role="dialog"
        onClick={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <div
          className={cn(
            DIALOG_SHELL_CLASS_NAME,
            "flex h-[min(80vh,720px)] w-full max-w-2xl flex-col overflow-hidden pointer-events-auto",
          )}
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          onPointerMove={(event) => event.stopPropagation()}
          onPointerUp={(event) => event.stopPropagation()}
        >
          <div className="dialog-header">
            <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
              <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-14 w-14 rounded-[20px] text-primary")}>
                <Hash className="h-7 w-7" />
              </div>
              <div className="min-w-0">
                <h2 className="dialog-title truncate" data-size="hero">
                  {resolved_dialog_title}
                </h2>
                <p className="dialog-subtitle truncate">
                  {resolved_dialog_subtitle}
                </p>
              </div>
            </div>
            <button
              aria-label={t("common.close")}
              className={DIALOG_ICON_BUTTON_CLASS_NAME}
              onClick={on_cancel}
              type="button"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* 内容区：左右两栏 */}
          <div className="dialog-body flex min-h-0 flex-1 gap-5 overflow-hidden">
            {/* 左栏：房间信息 */}
            <div className="flex min-h-0 w-60 shrink-0 flex-col gap-3">
              <p className="dialog-label">
                {t("room.settings_title")}
              </p>
              <div className="rounded-[18px] border border-(--divider-subtle-color) px-3.5 py-3">
                <div className="flex items-center gap-3">
                  <div className="flex h-11 w-11 items-center justify-center overflow-hidden rounded-[14px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
                    {preview_avatar_src ? (
                      <img
                        alt="room-avatar-preview"
                        className="h-full w-full object-contain"
                        src={preview_avatar_src}
                      />
                    ) : (
                      <Hash className="h-4.5 w-4.5 text-(--icon-default)" />
                    )}
                  </div>
                  <input
                    className="dialog-input min-w-0 flex-1 rounded-xl px-3 py-2 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                    maxLength={64}
                    onChange={(e) => set_room_name(e.target.value)}
                    onKeyDown={(e) => { if (e.key === "Enter" && can_create) handle_create(); }}
                    placeholder={t("room.name_required_placeholder")}
                    required
                    type="text"
                    value={room_name}
                  />
                </div>
                <IconPicker
                  class_name="mt-3"
                  disabled={is_creating}
                  layout="row"
                  icon_size="sm"
                  max_icons={12}
                  on_select={set_selected_avatar}
                  show_clear={false}
                  start_icon_id={13}
                  value={selected_avatar}
                />
              </div>

              <p className="dialog-label">
                {t("room.selected_members", { count: selected_ids.length, max: MAX_MEMBERS })}
              </p>

              <div
                className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto rounded-2xl border border-(--divider-subtle-color) p-2.5"
              >
                {selected_agents.length > 0 ? (
                  selected_agents.map((agent) => (
                    <div
                      key={agent.agent_id}
                      className="flex items-center gap-2 rounded-xl px-2.5 py-1.5 transition-colors hover:bg-black/3"
                    >
                      <WorkspaceIconFrame
                        class_name="h-6 w-6 overflow-hidden text-(--icon-default)"
                        shape="round"
                        size="sm"
                      >
                        {get_icon_avatar_src(agent.avatar) ? (
                          <img
                            alt={agent.name}
                            className="h-full w-full object-cover"
                            src={get_icon_avatar_src(agent.avatar) ?? undefined}
                          />
                        ) : (
                          get_initials(agent.name)
                        )}
                      </WorkspaceIconFrame>
                      <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-strong)">
                        {agent.name}
                      </span>
                      <button
                        className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--text-soft) transition-colors hover:text-red-500"
                        onClick={() => toggle_agent(agent.agent_id)}
                        type="button"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  ))
                ) : (
                  <p className="flex flex-1 items-center justify-center text-[12px] text-(--text-soft)">
                    {t("room.add_from_left")}
                  </p>
                )}
              </div>
            </div>

            {/* 右栏：Agent 列表 */}
            <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
              {/* 搜索框 */}
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-(--text-soft)" />
                <input
                  className="dialog-input w-full rounded-xl py-2 pl-8 pr-3 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                  onChange={(e) => set_search_query(e.target.value)}
                  placeholder={t("room.search_agent_placeholder")}
                  type="text"
                  value={search_query}
                />
              </div>

              <p className="dialog-label">
                {t("room.all_agents", { count: filtered_agents.length })}
              </p>

              <div className="flex min-h-0 flex-1 flex-col rounded-[18px] border border-(--divider-subtle-color) p-2.5">
                <div className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto pr-1">
                  {filtered_agents.map((agent) => {
                    const is_selected = selected_ids.includes(agent.agent_id);
                    return (
                      <button
                        key={agent.agent_id}
                        className={cn(
                          "dialog-card flex w-full items-center gap-3 rounded-[14px] px-3 py-2 text-left transition-all duration-(--motion-duration-normal)",
                          is_selected && "dialog-card-active",
                        )}
                        onClick={() => toggle_agent(agent.agent_id)}
                        type="button"
                      >
                        <WorkspaceIconFrame
                          class_name="overflow-hidden text-(--icon-default)"
                          shape="round"
                          size="sm"
                        >
                          {get_icon_avatar_src(agent.avatar) ? (
                            <img
                              alt={agent.name}
                              className="h-full w-full object-cover"
                              src={get_icon_avatar_src(agent.avatar) ?? undefined}
                            />
                          ) : (
                            <Bot className="h-4 w-4" />
                          )}
                        </WorkspaceIconFrame>

                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-semibold text-(--text-strong)">
                            {agent.name}
                          </p>
                          <p className="truncate text-[10px] text-(--text-muted)">
                            {agent.options?.system_prompt
                              ? agent.options.system_prompt.slice(0, 50) + (agent.options.system_prompt.length > 50 ? "..." : "")
                              : agent.status ?? t("status.idle")}
                          </p>
                        </div>

                        <div
                          className={cn(
                            "flex h-6 w-6 shrink-0 items-center justify-center rounded-full transition-all",
                            is_selected
                              ? "bg-primary text-white"
                              : "border border-(--surface-interactive-hover-border) text-(--text-soft)",
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
            </div>
          </div>

          {/* 底部栏 — 与 AgentOptions footer 一致 */}
          <div className="dialog-footer justify-end gap-3">
            {/* 操作按钮 */}
            <button
              className={get_dialog_action_class_name("default")}
              onClick={on_cancel}
              type="button"
            >
              {t("common.cancel")}
            </button>
            <button
              className={get_dialog_action_class_name(can_create ? "primary" : "default")}
              disabled={!can_create}
              onClick={handle_create}
              type="button"
            >
              {is_creating ? t("room.creating_action") : resolved_confirm_label}
            </button>
          </div>
        </div>
      </div>
    </>,
    document.body,
  );
}
