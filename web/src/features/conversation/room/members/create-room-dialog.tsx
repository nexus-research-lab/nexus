/**
 * 创建 Room 弹窗
 *
 * 复用 dialog-shell 设计系统，与 AgentOptions / SkillDetailDialog 风格统一。
 * 使用 createPortal 渲染到 document.body，确保全页面居中显示。
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Crown, Hash, Plus, Search } from "lucide-react";

import { get_available_skills_api } from "@/lib/api/skill-api";
import { cn } from "@/lib/utils";
import { ROOM_ICON_ID_END, ROOM_ICON_ID_START } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { UiMultiSelectMenu, UiSelectMenu } from "@/shared/ui/select-menu";
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
  initial_host_agent_id?: string | null;
  initial_host_auto_reply_enabled?: boolean;
  on_cancel: () => void;
  on_confirm: (
    agent_ids: string[],
    name: string,
    avatar?: string,
    skill_names?: string[],
    host_agent_id?: string | null,
    host_auto_reply_enabled?: boolean,
  ) => void;
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
  initial_host_agent_id = null,
  initial_host_auto_reply_enabled = false,
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
  const [room_skill_query, set_room_skill_query] = useState("");
  const [selected_host_agent_id, set_selected_host_agent_id] = useState<string>("");
  const [host_auto_reply_enabled, set_host_auto_reply_enabled] = useState(false);
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
      set_selected_host_agent_id(initial_host_agent_id?.trim() ?? "");
      set_host_auto_reply_enabled(initial_host_auto_reply_enabled);
      set_room_skill_query("");
    }
  }, [
    initial_avatar,
    initial_host_agent_id,
    initial_host_auto_reply_enabled,
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

  useEffect(() => {
    if (selected_ids.length === 0) {
      set_selected_host_agent_id("");
      set_host_auto_reply_enabled(false);
      return;
    }
    if (selected_host_agent_id && selected_ids.includes(selected_host_agent_id)) {
      return;
    }
    set_selected_host_agent_id("");
    set_host_auto_reply_enabled(false);
  }, [selected_host_agent_id, selected_ids]);

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
  const room_skill_options = useMemo(
    () => filtered_room_skills.map((skill) => ({
      value: skill.name,
      label: skill.name,
      description: skill.description || skill.title,
    })),
    [filtered_room_skills],
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

  const handle_change_host_agent = useCallback((agent_id: string) => {
    set_selected_host_agent_id(agent_id);
    if (!agent_id) {
      set_host_auto_reply_enabled(false);
    }
  }, []);

  const handle_create = useCallback(() => {
    if (selected_ids.length === 0 || !room_name.trim()) return;
    on_confirm(
      selected_ids,
      room_name.trim(),
      selected_avatar || undefined,
      selected_room_skill_names,
      selected_host_agent_id || null,
      host_auto_reply_enabled && selected_host_agent_id !== "",
    );
  }, [host_auto_reply_enabled, on_confirm, room_name, selected_avatar, selected_host_agent_id, selected_ids, selected_room_skill_names]);

  if (!is_open) return null;

  const can_create = selected_ids.length > 0 && room_name.trim().length > 0 && !is_creating;
  const resolved_dialog_title = dialog_title ?? (mode === "manage" ? t("room.manage_dialog_title") : t("room.create_dialog_title"));
  const resolved_dialog_subtitle = dialog_subtitle ?? (mode === "manage" ? t("room.manage_dialog_subtitle") : t("room.create_dialog_subtitle"));
  const resolved_confirm_label = confirm_label ?? (mode === "manage" ? t("common.save") : t("room.create_action"));

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9998]"
        labelled_by="create-room-dialog-title"
        on_close={on_cancel}
      >
        <UiDialogShell
          class_name="h-[min(80vh,720px)] pointer-events-auto"
          size="lg"
        >
          <UiDialogHeader>
            <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
              <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, "h-11 w-11 rounded-[16px] text-primary")}>
                <Hash className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2
                  className="dialog-title truncate"
                  id="create-room-dialog-title"
                >
                  {resolved_dialog_title}
                </h2>
                <p className="dialog-subtitle truncate">
                  {resolved_dialog_subtitle}
                </p>
              </div>
            </div>
            <UiDialogCloseButton on_close={on_cancel} />
          </UiDialogHeader>

          {/* 内容区：成员管理 + 底部 Room Skill 标签行 */}
          <div className="dialog-body flex min-h-0 flex-1 flex-col gap-4 overflow-hidden">
            <div className="flex min-h-0 flex-1 gap-5 overflow-hidden">
              {/* 左栏：房间信息 */}
              <div className="flex min-h-0 w-60 shrink-0 flex-col gap-3">
                <p className="dialog-label">
                  {t("room.settings_title")}
                </p>
                <div className="flex flex-col gap-3">
                  <div className="flex items-center gap-3">
                    <UiRoomAvatar
                      avatar={selected_avatar}
                      class_name="h-11 w-11 rounded-[14px]"
                      members={[]}
                      room_id={room_name}
                      title={room_name || resolved_dialog_title}
                    />
                    <input
                      className="dialog-input min-w-0 flex-1 rounded-xl px-3 py-2 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                      data-autofocus="true"
                      maxLength={64}
                      onChange={(e) => set_room_name(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" && can_create) {
                          handle_create();
                        }
                      }}
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

                <div className="flex flex-col gap-2">
                  <div className="flex items-center gap-2">
                    <div className="flex shrink-0 items-center gap-1.5 text-[11px] font-semibold text-(--text-muted)">
                      <Crown className="h-3.5 w-3.5 text-primary" />
                      <span>群主</span>
                    </div>
                    <UiSelectMenu
                      aria_label="选择 Room 群主"
                      class_name="min-w-0 flex-1"
                      disabled={selected_agents.length === 0 || is_creating}
                      on_change={handle_change_host_agent}
                      options={[
                        { value: "", label: "未设置" },
                        ...selected_agents.map((agent) => ({
                          value: agent.agent_id,
                          label: agent.name,
                        })),
                      ]}
                      size="sm"
                      surface="dialog"
                      value={selected_host_agent_id}
                    />
                  </div>
                  <label className="mt-1.5 flex items-center gap-2 px-0.5 text-[11px] font-medium text-(--text-default)">
                    <input
                      checked={host_auto_reply_enabled}
                      className="h-3.5 w-3.5 shrink-0 accent-[var(--primary)] disabled:cursor-not-allowed disabled:opacity-55"
                      disabled={!selected_host_agent_id || is_creating}
                      onChange={(event) => set_host_auto_reply_enabled(event.target.checked)}
                      type="checkbox"
                    />
                    <span className="min-w-0 truncate">
                      未 @ 时由群主接管，可回答或委派
                    </span>
                  </label>
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

                <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-[16px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_84%,transparent)] px-2 py-2">
                  <div className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto pr-1">
                    {filtered_agents.map((agent) => {
                      const is_selected = selected_id_set.has(agent.agent_id);
                      const action_label = is_selected
                        ? t("room.agent_select_remove", { name: agent.name })
                        : t("room.agent_select_add", { name: agent.name });
                      return (
                        <button
                          aria-label={action_label}
                          aria-pressed={is_selected}
                          key={agent.agent_id}
                          className={cn(
                            "flex w-full cursor-pointer items-center gap-3 rounded-[14px] border px-3 py-1.5 text-left transition-[background,border-color] duration-(--motion-duration-normal)",
                            is_selected
                              ? "border-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_13%,transparent)]"
                              : "border-[color:color-mix(in_srgb,var(--divider-subtle-color)_58%,transparent)] bg-transparent hover:border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)]",
                          )}
                          onClick={() => toggle_agent(agent.agent_id)}
                          title={action_label}
                          type="button"
                        >
                          <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />

                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-semibold text-(--text-strong)">
                              {agent.name}
                            </p>
                          </div>

                          <div
                            className={cn(
                              "pointer-events-none flex h-6 w-6 shrink-0 items-center justify-center rounded-full transition-all",
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

            <UiMultiSelectMenu
              aria_label={t("room.skills_label")}
              class_name="shrink-0"
              disabled={is_creating}
              empty_text={t("room.skills_empty")}
              error_text={room_skill_error}
              is_loading={is_loading_room_skills}
              loading_text={t("room.skills_loading")}
              on_change={set_selected_room_skill_names}
              on_query_change={set_room_skill_query}
              options={room_skill_options}
              placement="top"
              placeholder={t("room.skills_none")}
              query={room_skill_query}
              search_placeholder={t("agent_options.skills.search_placeholder")}
              surface="dialog"
              value={selected_room_skill_names}
            />
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
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
