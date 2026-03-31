"use client";

import { useEffect, useState } from "react";
import { Settings, Trash2, X } from "lucide-react";

import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
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
        className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-150"
        role="dialog"
        aria-modal="true"
      >
        <div className="soft-ring radius-shell-lg panel-surface w-full max-w-lg p-6 animate-in zoom-in-95 duration-150">
          {/* 标题栏 */}
          <div className="flex items-start justify-between gap-3 pb-4">
            <div className="flex items-center gap-2">
              <Settings className="h-4.5 w-4.5 text-slate-700" />
              <h2 className="text-lg font-semibold text-slate-900">
                Room 设置
              </h2>
            </div>
            <button
              aria-label="关闭"
              className="neo-pill radius-shell-sm p-1.5 text-slate-500 transition-colors hover:text-slate-700"
              onClick={on_close}
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          {/* 内容区 */}
          <div className="space-y-4 pb-6">
            {/* 名称设置 */}
            <div className="space-y-2">
              <label className="text-xs font-semibold uppercase tracking-wider text-slate-700/64">
                名称
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
                    placeholder="输入 Room 名称"
                    maxLength={64}
                    className="neo-inset radius-shell-sm flex-1 px-3.5 py-2.5 text-sm text-slate-900 placeholder:text-slate-400/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none"
                  />
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => handle_update_name(edit_name_value)}
                    disabled={is_updating}
                  >
                    保存
                  </WorkspacePillButton>
                </div>
              ) : (
                <div className="flex items-center justify-between gap-3 rounded-lg bg-slate-50/50 px-4 py-3">
                  <p className="text-sm text-slate-900">{room_name || "未命名 Room"}</p>
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => set_is_editing_name(true)}
                  >
                    编辑
                  </WorkspacePillButton>
                </div>
              )}
            </div>

            {/* 描述设置 */}
            <div className="space-y-2">
              <label className="text-xs font-semibold uppercase tracking-wider text-slate-700/64">
                描述
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
                    placeholder="描述这个 Room 的用途..."
                    maxLength={256}
                    rows={3}
                    className="neo-inset radius-shell-sm flex-1 px-3.5 py-2.5 text-sm text-slate-900 placeholder:text-slate-400/50 focus-visible:ring-2 focus-visible:ring-primary/20 focus-visible:outline-none resize-none"
                  />
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => handle_update_description(edit_description_value)}
                    disabled={is_updating}
                  >
                    保存
                  </WorkspacePillButton>
                </div>
              ) : (
                <div className="flex items-center justify-between gap-3 rounded-lg bg-slate-50/50 px-4 py-3">
                  <p className="text-sm text-slate-900 min-w-0 flex-1 truncate">
                    {room_description || "暂无描述"}
                  </p>
                  <WorkspacePillButton
                    size="sm"
                    onClick={() => set_is_editing_description(true)}
                  >
                    编辑
                  </WorkspacePillButton>
                </div>
              )}
            </div>

            {/* 危险操作区 */}
            <div className="pt-4 border-t border-slate-200/40">
              <WorkspacePillButton
                variant="danger"
                size="md"
                onClick={() => set_is_delete_confirm_open(true)}
                class_name="w-full justify-center"
              >
                <Trash2 className="h-4 w-4" />
                删除 Room
              </WorkspacePillButton>
            </div>
          </div>
        </div>
      </div>

      {/* 删除确认对话框 */}
      <ConfirmDialog
        is_open={is_delete_confirm_open}
        title="删除 Room"
        message="确定要删除这个 Room 吗？此操作将移除 Room 及其所有对话和消息，无法恢复。"
        confirm_text="删除"
        cancel_text="取消"
        variant="danger"
        on_confirm={handle_delete_room}
        on_cancel={() => set_is_delete_confirm_open(false)}
      />
    </>
  );
}
