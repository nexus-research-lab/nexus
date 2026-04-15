"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Settings, Trash2, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { get_icon_avatar_src } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  get_dialog_action_class_name,
  get_dialog_note_class_name,
  get_dialog_note_style,
} from "@/shared/ui/dialog/dialog-styles";
import { UpdateRoomParams } from "@/types/room";

interface RoomSettingsPanelProps {
  is_open: boolean;
  room_id: string | null;
  room_name: string;
  room_description: string;
  room_avatar?: string | null;
  fallback_avatar: string;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_close: () => void;
}

export function RoomSettingsPanel({
  is_open,
  room_id,
  room_name,
  room_description,
  room_avatar = "",
  fallback_avatar,
  on_update_room,
  on_delete_room,
  on_close,
}: RoomSettingsPanelProps) {
  const { t } = useI18n();
  const [is_editing_name, set_is_editing_name] = useState(false);
  const [is_editing_description, set_is_editing_description] = useState(false);
  const [is_delete_confirm_open, set_is_delete_confirm_open] = useState(false);
  const [edit_name_value, set_edit_name_value] = useState(room_name);
  const [edit_description_value, set_edit_description_value] = useState(room_description);
  const [edit_avatar_value, set_edit_avatar_value] = useState(room_avatar?.trim() ?? "");
  const [is_updating, set_is_updating] = useState(false);

  // 当对话框打开时重置编辑值
  useEffect(() => {
    if (is_open) {
      set_edit_name_value(room_name);
      set_edit_description_value(room_description);
      set_edit_avatar_value(room_avatar?.trim() ?? "");
    }
  }, [is_open, room_name, room_description, room_avatar]);

  const handle_update_name = async (value: string) => {
    if (!room_id || value.trim() === room_name) {
      set_edit_name_value(room_name);
      return;
    }
    set_is_updating(true);
    try {
      await on_update_room(room_id, {
        name: value.trim(),
        description: edit_description_value,
        avatar: edit_avatar_value,
      });
    } finally {
      set_is_updating(false);
    }
    set_is_editing_name(false);
  };

  const handle_update_description = async (value: string) => {
    if (!room_id || value.trim() === room_description) {
      set_edit_description_value(room_description);
      return;
    }
    set_is_updating(true);
    try {
      await on_update_room(room_id, {
        name: edit_name_value,
        description: value.trim(),
        avatar: edit_avatar_value,
      });
    } finally {
      set_is_updating(false);
    }
    set_is_editing_description(false);
  };

  const handle_update_avatar = async (avatar_id: string) => {
    const normalized_avatar = room_avatar?.trim() ?? "";
    if (!room_id || avatar_id === normalized_avatar) {
      return;
    }
    set_is_updating(true);
    try {
      await on_update_room(room_id, {
        name: edit_name_value.trim() || room_name,
        description: edit_description_value,
        avatar: avatar_id || "",
      });
      set_edit_avatar_value(avatar_id);
    } finally {
      set_is_updating(false);
    }
  };

  const preview_avatar_src = get_icon_avatar_src(edit_avatar_value || fallback_avatar);

  const handle_delete_room = async () => {
    set_is_delete_confirm_open(false);
    await on_delete_room();
  };

  if (!is_open || typeof document === "undefined") return null;

  return createPortal(
    <>
      <div
        aria-hidden="true"
        className={cn(DIALOG_BACKDROP_CLASS_NAME, "z-[9998]")}
        onClick={on_close}
      />
      <div
        data-modal-root="true"
        aria-modal="true"
        className="fixed inset-0 z-[9999] flex items-center justify-center p-6"
        role="dialog"
        onPointerDown={(event) => event.stopPropagation()}
        onPointerMove={(event) => event.stopPropagation()}
        onPointerUp={(event) => event.stopPropagation()}
      >
        <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-(--motion-duration-fast)">
          <div className="dialog-header">
            <div className={DIALOG_HEADER_LEADING_CLASS_NAME}>
              <div className={DIALOG_HEADER_ICON_CLASS_NAME}>
                <Settings className="h-4.5 w-4.5" />
              </div>
              <div className="min-w-0">
                <h2 className="dialog-title">{t("room.settings_title")}</h2>
                <p className="dialog-subtitle">
                  调整房间名称、描述和成员协作信息。
                </p>
              </div>
            </div>
            <button
              aria-label={t("common.close")}
              className={DIALOG_ICON_BUTTON_CLASS_NAME}
              onClick={on_close}
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="dialog-body space-y-4">
            <section className="dialog-card rounded-[20px] p-4">
              <div className="space-y-2">
                <label className="dialog-label">
                  {t("room.name")}
                </label>
                {is_editing_name ? (
                  <div className="space-y-3">
                    <input
                      autoFocus
                      type="text"
                      value={edit_name_value}
                      onChange={(e) => set_edit_name_value(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Escape") {
                          set_edit_name_value(room_name);
                          set_is_editing_name(false);
                        }
                      }}
                      placeholder={t("room.name_placeholder")}
                      maxLength={64}
                      className="dialog-input radius-shell-sm w-full px-3.5 py-2.5 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                    />
                    <div className="flex items-center justify-end gap-2">
                      <button
                        className={get_dialog_action_class_name("default")}
                        disabled={is_updating}
                        onClick={() => {
                          set_edit_name_value(room_name);
                          set_is_editing_name(false);
                        }}
                        type="button"
                      >
                        {t("common.cancel")}
                      </button>
                      <button
                        className={get_dialog_action_class_name("primary")}
                        disabled={is_updating}
                        onClick={() => handle_update_name(edit_name_value)}
                        type="button"
                      >
                        {t("common.save")}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between gap-3 rounded-[16px] border border-(--divider-subtle-color) px-4 py-3.5">
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-semibold text-(--text-strong)">
                        {room_name || t("room.untitled_room")}
                      </p>
                    </div>
                    <button
                      className="text-[11px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]"
                      onClick={() => set_is_editing_name(true)}
                      type="button"
                    >
                      {t("common.edit")}
                    </button>
                  </div>
                )}
              </div>
            </section>

            <section className="dialog-card rounded-[20px] p-4">
              <div className="space-y-2">
                <label className="dialog-label">
                  {t("room.description")}
                </label>
                {is_editing_description ? (
                  <div className="space-y-3">
                    <textarea
                      value={edit_description_value}
                      onChange={(e) => set_edit_description_value(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Escape") {
                          set_edit_description_value(room_description);
                          set_is_editing_description(false);
                        }
                      }}
                      placeholder={t("room.description_placeholder")}
                      maxLength={256}
                      rows={4}
                      className="dialog-input radius-shell-sm w-full resize-none px-3.5 py-2.5 text-sm text-(--text-strong) placeholder:text-(--text-soft) focus-visible:outline-none"
                    />
                    <div className="flex items-center justify-end gap-2">
                      <button
                        className={get_dialog_action_class_name("default")}
                        disabled={is_updating}
                        onClick={() => {
                          set_edit_description_value(room_description);
                          set_is_editing_description(false);
                        }}
                        type="button"
                      >
                        {t("common.cancel")}
                      </button>
                      <button
                        className={get_dialog_action_class_name("primary")}
                        disabled={is_updating}
                        onClick={() => handle_update_description(edit_description_value)}
                        type="button"
                      >
                        {t("common.save")}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between gap-3 rounded-[16px] border border-(--divider-subtle-color) px-4 py-3.5">
                    <div className="min-w-0 flex-1">
                      <p className="text-sm text-(--text-strong)">
                        {room_description || t("room.no_description")}
                      </p>
                    </div>
                    <button
                      className="text-[11px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]"
                      onClick={() => set_is_editing_description(true)}
                      type="button"
                    >
                      {t("common.edit")}
                    </button>
                  </div>
                )}
              </div>
            </section>

            <section className="dialog-card rounded-[20px] p-4">
              <div className="space-y-3">
                <label className="dialog-label">
                  {t("room.avatar")}
                </label>
                <div className="space-y-2">
                  <div className="flex items-center gap-3 rounded-[16px] border border-(--divider-subtle-color) px-4 py-3.5">
                    <div className="flex h-10 w-10 items-center justify-center overflow-hidden rounded-[12px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)">
                      <img
                        alt="room-avatar"
                        className="h-full w-full object-contain"
                        crossOrigin="anonymous"
                        src={preview_avatar_src ?? undefined}
                      />
                    </div>
                    <div className="min-w-0 flex-1">
                    </div>
                  </div>
                  <IconPicker
                    value={edit_avatar_value}
                    on_select={handle_update_avatar}
                    max_icons={12}
                    start_icon_id={13}
                    columns={6}
                    icon_size="md"
                    show_clear
                    disabled={is_updating}
                  />
                </div>
              </div>
            </section>

            <section
              className={get_dialog_note_class_name("danger")}
              style={get_dialog_note_style("danger")}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-(--text-strong)">
                    {t("room.delete_title")}
                  </p>
                  <p className="mt-1 text-sm leading-6 text-(--text-default)">
                    删除后会移除这个房间及其会话入口，请确认当前成员已经完成迁移。
                  </p>
                </div>
                <button
                  className={get_dialog_action_class_name("danger", "shrink-0")}
                  onClick={() => set_is_delete_confirm_open(true)}
                  type="button"
                >
                  <Trash2 className="h-4 w-4" />
                  {t("common.delete")}
                </button>
              </div>
            </section>
          </div>

          <div className="dialog-footer">
            <button
              className={get_dialog_action_class_name("default")}
              onClick={on_close}
              type="button"
            >
              {t("common.close")}
            </button>
          </div>
        </div>
      </div>

      {/* 删除确认对话框 */}
      <ConfirmDialog
        is_open={is_delete_confirm_open}
        title={t("room.delete_title")}
        message={t("room.delete_message")}
        confirm_text={t("common.delete")}
        cancel_text={t("common.cancel")}
        variant="danger"
        on_confirm={handle_delete_room}
        on_cancel={() => set_is_delete_confirm_open(false)}
      />
    </>,
    document.body,
  );
}
