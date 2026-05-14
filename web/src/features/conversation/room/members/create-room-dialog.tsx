/**
 * 创建 Room 弹窗
 *
 * 复用 dialog-shell 设计系统，与 AgentOptions / SkillDetailDialog 风格统一。
 * 使用 createPortal 渲染到 document.body，确保全页面居中显示。
 */

"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Bot, Check, ChevronDown, Hash, Loader2, Plus, Search, X } from "lucide-react";

import { get_available_skills_api } from "@/lib/api/skill-api";
import { cn } from "@/lib/utils";
import { get_icon_avatar_src, get_initials, get_room_avatar_icon_id, ROOM_ICON_ID_END, ROOM_ICON_ID_START } from "@/lib/utils";
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
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import type { SkillInfo } from "@/types/capability/skill";

export interface RoomMemberAgentOption {
  agent_id: string;
  name: string;
  avatar?: string | null;
  status?: string;
  headline?: string | null;
  description?: string | null;
}

interface CreateRoomDialogProps {
  agents: RoomMemberAgentOption[];
  is_open: boolean;
  is_creating?: boolean;
  mode?: "create" | "manage";
  dialog_title?: string;
  dialog_subtitle?: string;
  confirm_label?: string;
  initial_name?: string;
  initial_avatar?: string;
  initial_selected_agent_ids?: string[];
  initial_room_skill_names?: string[];
  on_cancel: () => void;
  on_confirm: (agent_ids: string[], name: string, avatar?: string, skill_names?: string[]) => void;
}

const MAX_MEMBERS = 10;
const EMPTY_STRING_LIST: string[] = [];
const STRING_LIST_SIGNATURE_SEPARATOR = "\x1f";

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
  initial_room_skill_names,
  on_cancel,
  on_confirm,
}: CreateRoomDialogProps) {
  const { t } = useI18n();
  const [search_query, set_search_query] = useState("");
  const [selected_ids, set_selected_ids] = useState<string[]>([]);
  const [room_name, set_room_name] = useState("");
  const [selected_avatar, set_selected_avatar] = useState("");
  const [selected_room_skill_names, set_selected_room_skill_names] = useState<string[]>([]);
  const [available_room_skills, set_available_room_skills] = useState<SkillInfo[]>([]);
  const [is_loading_room_skills, set_is_loading_room_skills] = useState(false);
  const [room_skill_error, set_room_skill_error] = useState<string | null>(null);
  const [is_room_skill_menu_open, set_is_room_skill_menu_open] = useState(false);
  const [room_skill_query, set_room_skill_query] = useState("");
  const room_skill_selector_ref = useRef<HTMLDivElement | null>(null);
  const normalized_initial_selected_ids = initial_selected_agent_ids ?? EMPTY_STRING_LIST;
  const normalized_initial_room_skill_names = initial_room_skill_names ?? EMPTY_STRING_LIST;
  // 数组 props 往往每次 render 都是新引用，依赖内容签名，
  // 避免弹窗打开时因默认空数组或父层重建数组而反复 setState。
  const initial_selected_ids_signature = useMemo(
    () => normalized_initial_selected_ids.join(STRING_LIST_SIGNATURE_SEPARATOR),
    [normalized_initial_selected_ids],
  );
  const stable_initial_selected_ids = useMemo(
    () =>
      initial_selected_ids_signature === ""
        ? []
        : initial_selected_ids_signature.split(STRING_LIST_SIGNATURE_SEPARATOR),
    [initial_selected_ids_signature],
  );
  const initial_room_skill_names_signature = useMemo(
    () => normalized_initial_room_skill_names.join(STRING_LIST_SIGNATURE_SEPARATOR),
    [normalized_initial_room_skill_names],
  );
  const stable_initial_room_skill_names = useMemo(
    () =>
      initial_room_skill_names_signature === ""
        ? []
        : initial_room_skill_names_signature.split(STRING_LIST_SIGNATURE_SEPARATOR),
    [initial_room_skill_names_signature],
  );

  // 打开时重置状态
  useEffect(() => {
    if (is_open) {
      set_search_query("");
      set_selected_ids(stable_initial_selected_ids);
      set_room_name(initial_name);
      set_selected_avatar(initial_avatar);
      set_selected_room_skill_names(stable_initial_room_skill_names);
      set_is_room_skill_menu_open(false);
      set_room_skill_query("");
    }
  }, [
    initial_avatar,
    initial_name,
    initial_room_skill_names_signature,
    initial_selected_ids_signature,
    is_open,
    stable_initial_room_skill_names,
    stable_initial_selected_ids,
  ]);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    let is_cancelled = false;
    set_is_loading_room_skills(true);
    set_room_skill_error(null);
    get_available_skills_api({scope: "room"})
      .then((items) => {
        if (!is_cancelled) {
          set_available_room_skills(items);
        }
      })
      .catch((error: unknown) => {
        if (!is_cancelled) {
          set_room_skill_error(error instanceof Error ? error.message : t("room.skills_load_error"));
        }
      })
      .finally(() => {
        if (!is_cancelled) {
          set_is_loading_room_skills(false);
        }
      });
    return () => {
      is_cancelled = true;
    };
  }, [is_open, t]);

  // ESC 关闭
  useEffect(() => {
    if (!is_open) return;
    const handle = (e: KeyboardEvent) => {
      if (e.key === "Escape") on_cancel();
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, [is_open, on_cancel]);

  useEffect(() => {
    if (!is_open || !is_room_skill_menu_open) {
      return;
    }
    const handle_pointer_down = (event: PointerEvent) => {
      const target = event.target;
      if (
        target instanceof Node
        && room_skill_selector_ref.current?.contains(target)
      ) {
        return;
      }
      set_is_room_skill_menu_open(false);
    };
    document.addEventListener("pointerdown", handle_pointer_down, true);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down, true);
    };
  }, [is_open, is_room_skill_menu_open]);

  // 搜索过滤
  const filtered_agents = useMemo(() => {
    if (!search_query.trim()) return agents;
    const q = search_query.toLowerCase();
    return agents.filter((a) => a.name.toLowerCase().includes(q));
  }, [agents, search_query]);

  // 已选中的 Agent 对象列表
  const selected_id_set = useMemo(() => new Set(selected_ids), [selected_ids]);
  const selected_agents = useMemo(
    () => agents.filter((a) => selected_id_set.has(a.agent_id)),
    [agents, selected_id_set],
  );

  const selected_room_skill_name_set = useMemo(
    () => new Set(selected_room_skill_names),
    [selected_room_skill_names],
  );
  const filtered_room_skills = useMemo(() => {
    const query = room_skill_query.trim().toLowerCase();
    if (!query) {
      return available_room_skills;
    }
    return available_room_skills.filter((skill) =>
      skill.name.toLowerCase().includes(query)
      || skill.title.toLowerCase().includes(query)
      || skill.description.toLowerCase().includes(query),
    );
  }, [available_room_skills, room_skill_query]);

  const toggle_agent = useCallback((agent_id: string) => {
    set_selected_ids((prev) => {
      if (prev.includes(agent_id)) {
        return prev.filter((id) => id !== agent_id);
      }
      if (prev.length >= MAX_MEMBERS) return prev;
      return [...prev, agent_id];
    });
  }, []);

  const toggle_room_skill = useCallback((skill_name: string) => {
    set_selected_room_skill_names((prev) => {
      if (prev.includes(skill_name)) {
        return prev.filter((name) => name !== skill_name);
      }
      return [...prev, skill_name];
    });
  }, []);

  const remove_room_skill = useCallback((skill_name: string) => {
    set_selected_room_skill_names((prev) => prev.filter((name) => name !== skill_name));
  }, []);

  const handle_room_skill_trigger_key_down = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    set_is_room_skill_menu_open((current) => !current);
  }, []);

  const handle_create = useCallback(() => {
    if (selected_ids.length === 0 || !room_name.trim()) return;
    on_confirm(selected_ids, room_name.trim(), selected_avatar || undefined, selected_room_skill_names);
  }, [on_confirm, room_name, selected_avatar, selected_ids, selected_room_skill_names]);

  if (!is_open) return null;

  const can_create = selected_ids.length > 0 && room_name.trim().length > 0 && !is_creating;
  const preview_avatar_id = get_room_avatar_icon_id(null, room_name, selected_avatar);
  const preview_avatar_src = get_icon_avatar_src(preview_avatar_id, "room");
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

          {/* 内容区：成员管理 + 底部 Room Skill 标签行 */}
          <div className="dialog-body flex min-h-0 flex-1 flex-col gap-4 overflow-hidden">
            <div className="flex min-h-0 flex-1 gap-5 overflow-hidden">
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
                          className="h-full w-full object-cover"
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
                    icon_family="room"
                    layout="row"
                    icon_size="sm"
                    max_icons={ROOM_ICON_ID_END - ROOM_ICON_ID_START + 1}
                    on_select={set_selected_avatar}
                    show_clear={false}
                    start_icon_id={ROOM_ICON_ID_START}
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
                      const is_selected = selected_id_set.has(agent.agent_id);
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
                              {agent.headline?.trim()
                                || agent.description?.trim()
                                || agent.status
                                || t("status.idle")}
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

            <div className="relative shrink-0" ref={room_skill_selector_ref}>
              {is_room_skill_menu_open ? (
                <div
                  className="absolute bottom-full left-0 right-0 z-50 mb-1.5 overflow-hidden rounded-[18px] border border-(--modal-card-border) bg-(--modal-dialog-body-background) shadow-[0_18px_48px_rgba(15,23,42,0.2)]"
                  role="listbox"
                >
                  <div className="flex h-10 items-center gap-2 border-b border-(--divider-subtle-color) bg-(--modal-dialog-header-background) px-3">
                    <Search className="h-3.5 w-3.5 shrink-0 text-(--text-soft)" />
                    <input
                      className="min-w-0 flex-1 appearance-none bg-transparent text-[13px] font-medium text-(--text-strong) outline-none ring-0 placeholder:text-(--text-soft) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0"
                      onChange={(event) => set_room_skill_query(event.target.value)}
                      onClick={(event) => event.stopPropagation()}
                      placeholder={t("agent_options.skills.search_placeholder")}
                      type="text"
                      value={room_skill_query}
                    />
                  </div>
                  {is_loading_room_skills ? (
                    <div className="flex h-12 items-center gap-2 px-3 text-sm text-(--text-soft)">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      {t("room.skills_loading")}
                    </div>
                  ) : room_skill_error ? (
                    <div className="m-2 rounded-xl border border-red-200 px-3 py-2.5 text-sm text-red-700">
                      {room_skill_error}
                    </div>
                  ) : filtered_room_skills.length === 0 ? (
                    <div className="flex h-12 items-center px-3 text-sm text-(--text-soft)">
                      {t("room.skills_empty")}
                    </div>
                  ) : (
                    <div className="soft-scrollbar max-h-52 overflow-y-auto py-1">
                      {filtered_room_skills.map((skill) => {
                        const checked = selected_room_skill_name_set.has(skill.name);
                        return (
                          <button
                            aria-selected={checked}
                            className={cn(
                              "flex h-8 w-full items-center gap-2.5 px-3 text-left text-[13px] font-medium transition duration-(--motion-duration-fast)",
                              checked
                                ? "bg-(--modal-input-focus-background) text-(--text-strong)"
                                : "text-(--text-default) hover:bg-(--modal-input-background) hover:text-(--text-strong)",
                            )}
                            key={skill.name}
                            onClick={() => toggle_room_skill(skill.name)}
                            role="option"
                            type="button"
                          >
                            <span className="min-w-0 flex-1 truncate">
                              {skill.name}
                            </span>
                            <span className="flex h-4 w-4 shrink-0 items-center justify-center text-(--text-default)">
                              {checked ? <Check className="h-3.5 w-3.5" /> : null}
                            </span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              ) : null}
              <div
                aria-label={t("room.skills_label")}
                aria-expanded={is_room_skill_menu_open}
                aria-haspopup="listbox"
                className={cn(
                  "flex min-h-11 w-full items-center gap-2 rounded-[16px] border bg-(--modal-input-background) px-3.5 py-2 text-left shadow-[inset_0_1px_0_rgba(255,255,255,0.42)] transition duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]",
                  is_room_skill_menu_open
                    ? "border-[color:color-mix(in_srgb,var(--primary)_30%,var(--divider-subtle-color))] bg-(--modal-input-focus-background) ring-1 ring-inset ring-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]"
                    : "border-[color:color-mix(in_srgb,var(--modal-input-border)_88%,transparent)] hover:border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color))] hover:bg-(--modal-input-focus-background) hover:ring-1 hover:ring-inset hover:ring-[color:color-mix(in_srgb,var(--primary)_10%,transparent)]",
                )}
                onKeyDown={handle_room_skill_trigger_key_down}
                onClick={() => set_is_room_skill_menu_open((current) => !current)}
                role="button"
                tabIndex={0}
              >
                <span className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
                  {selected_room_skill_names.length > 0 ? (
                    selected_room_skill_names.map((skill_name) => (
                      <span
                        className="inline-flex max-w-[11rem] items-center gap-1 rounded-full border border-(--divider-subtle-color) bg-(--modal-dialog-body-background) py-0.5 pl-2 pr-1 text-[11px] font-semibold text-(--text-strong)"
                        key={skill_name}
                      >
                        <span className="min-w-0 truncate">
                          {skill_name}
                        </span>
                        <button
                          aria-label={`移除 ${skill_name}`}
                          className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--text-soft) transition duration-(--motion-duration-fast) hover:bg-(--modal-input-background) hover:text-(--text-strong)"
                          onClick={(event) => {
                            event.stopPropagation();
                            remove_room_skill(skill_name);
                          }}
                          onKeyDown={(event) => event.stopPropagation()}
                          type="button"
                        >
                          <X className="h-2.5 w-2.5" />
                        </button>
                      </span>
                    ))
                  ) : (
                    <span className="rounded-full border border-dashed border-(--divider-subtle-color) bg-(--modal-dialog-body-background) px-2 py-0.5 text-[11px] font-medium text-(--text-soft)">
                      {t("room.skills_none")}
                    </span>
                  )}
                </span>
                {is_loading_room_skills ? (
                  <Loader2 className="h-4 w-4 shrink-0 animate-spin text-(--text-soft)" />
                ) : (
                  <ChevronDown
                    className={cn(
                      "h-4 w-4 shrink-0 text-(--text-soft) transition-transform duration-(--motion-duration-fast)",
                      is_room_skill_menu_open && "rotate-180",
                    )}
                  />
                )}
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
