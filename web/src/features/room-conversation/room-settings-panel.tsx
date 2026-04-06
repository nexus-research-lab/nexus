"use client";

import { useEffect, useState } from "react";
import { Settings, Trash2, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { UpdateRoomParams } from "@/types/room";

interface RoomSettingsPanelProps {
  is_open: boolean;
  room_id: string | null;
  room_name: string;
  room_description: string;
  on_update_room: (room_id: string, params: UpdateRoomParams) => Promise<void>;
  on_delete_room: () => Promise<void>;
  on_close: () => void;
}

export function RoomSettingsPanel({
  is_open,
  room_id,
  room_name,
  room_description,
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
  const [is_updating, set_is_updating] = useState(false);

  // 当对话框打开时重置编辑值
  useEffect(() => {
    if (is_open) {
      set_edit_name_value(room_name);
      set_edit_description_value(room_description);
    }
  }, [is_open, room_name, room_description]);

  const handle_update_name = async (value: string) => {
    if (!room_id || value.trim() === room_name) {
      set_edit_name_value(room_name);
      return;
    }
    set_is_updating(true);
    try {
      await on_update_room(room_id, {
        name: value.trim(),
        description: room_description,
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
        name: room_name,
        description: value.trim(),
      });
    } finally {
      set_is_updating(false);
    }
    set_is_editing_description(false);
  };

  const handle_delete_room = async () => {
    set_is_delete_confirm_open(false);
    await on_delete_room();
  };

  if (!is_open) return null;

  return (
    <>
      <div
        className="dialog-backdrop animate-in fade-in duration-150"
        role="dialog"
        aria-modal="true"
      >
          <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-150">
            <div className="dialog-header">
              <div className={DIALOG_HEADER_LEADING_CLASS_NAME}>
                <div className={DIALOG_HEADER_ICON_CLASS_NAME}>
                  <Settings className="h-4.5 w-4.5" />
                </div>
              <div className="min-w-0">
                <h2 className="dialog-title">
                  {t("room.settings_title")}
                </h2>
                <p className="dialog-subtitle">
                  调整房间名称、描述和成员协作信息。
                </p>
              </div>
            </div>
            <WorkspacePillButton
              aria-label={t("common.close")}
              density="compact"
              onClick={on_close}
              size="icon"
              variant="icon"
            >
              <X className="h-4 w-4" />
            </WorkspacePillButton>
          </div>

          <div className="dialog-body space-y-4">
            <div className="space-y-2">
              <label className="dialog-label">
                {t("room.name")}
              </label>
              {is_editing_name ? (
                <div className="flex gap-2">
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
                    className="dialog-input radius-shell-sm flex-1 px-3.5 py-2.5 text-sm text-slate-900 placeholder:text-slate-400/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
                  />
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => handle_update_name(edit_name_value)}
                    disabled={is_updating}
                    variant="primary"
                  >
                    {t("common.save")}
                  </WorkspacePillButton>
                </div>
              ) : (
                <div className="surface-inset radius-shell-sm flex items-center justify-between gap-3 px-4 py-3">
                  <p className="text-sm text-slate-900">{room_name || t("room.untitled_room")}</p>
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => set_is_editing_name(true)}
                  >
                    {t("common.edit")}
                  </WorkspacePillButton>
                </div>
              )}
            </div>

            <div className="space-y-2">
              <label className="dialog-label">
                {t("room.description")}
              </label>
              {is_editing_description ? (
                <div className="flex gap-2">
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
                    rows={3}
                    className="dialog-input radius-shell-sm flex-1 resize-none px-3.5 py-2.5 text-sm text-slate-900 placeholder:text-slate-400/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
                  />
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => handle_update_description(edit_description_value)}
                    disabled={is_updating}
                    variant="primary"
                  >
                    {t("common.save")}
                  </WorkspacePillButton>
                </div>
              ) : (
                <div className="surface-inset radius-shell-sm flex items-center justify-between gap-3 px-4 py-3">
                  <p className="min-w-0 flex-1 truncate text-sm text-slate-900">
                    {room_description || t("room.no_description")}
                  </p>
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => set_is_editing_description(true)}
                  >
                    {t("common.edit")}
                  </WorkspacePillButton>
                </div>
              )}
            </div>

            <div className="dialog-footer px-0 pb-0">
              <WorkspacePillButton
                tone="danger"
                variant="outlined"
                size="md"
                onClick={() => set_is_delete_confirm_open(true)}
                stretch
              >
                <Trash2 className="h-4 w-4" />
                {t("room.delete_title")}
              </WorkspacePillButton>
            </div>
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
    </>
  );
}
