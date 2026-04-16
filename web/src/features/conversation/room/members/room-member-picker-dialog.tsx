"use client";

import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { Agent } from "@/types/agent/agent";
import { cn } from "@/lib/utils";
import { get_icon_avatar_src, get_initials } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  DIALOG_EMPTY_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";

interface RoomMemberPickerDialogProps {
  agents: Agent[];
  is_open: boolean;
  on_cancel: () => void;
  on_select: (agent_id: string) => void;
}

export function RoomMemberPickerDialog({
  agents,
  is_open,
  on_cancel,
  on_select,
}: RoomMemberPickerDialogProps) {
  const { t } = useI18n();
  if (!is_open) {
    return null;
  }

  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <>
      <div
        aria-hidden="true"
        className="dialog-backdrop z-[9998]"
        onClick={on_cancel}
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
        <div className="dialog-shell radius-shell-lg w-full max-w-lg">
          <div className="dialog-header">
            <div className="min-w-0 flex-1">
              <h3 className="dialog-title">{t("room.add_member_dialog_title")}</h3>
              <p className="dialog-subtitle">
                {t("room.add_member_dialog_subtitle")}
              </p>
            </div>
            <button
              aria-label={t("common.close")}
              className={DIALOG_ICON_BUTTON_CLASS_NAME}
              onClick={on_cancel}
              type="button"
            >
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="dialog-body">
            {agents.length === 0 ? (
              <div className={DIALOG_EMPTY_CLASS_NAME}>
                {t("room.no_available_members")}
              </div>
            ) : (
              <div className="max-h-[360px] space-y-2 overflow-y-auto pr-1">
                {agents.map((agent) => (
                  <button
                    key={agent.agent_id}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-[20px] border border-(--divider-subtle-color) px-4 py-3 text-left transition-colors duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)",
                    )}
                    onClick={() => on_select(agent.agent_id)}
                    type="button"
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
                      {get_icon_avatar_src(agent.avatar) ? (
                        <img
                          alt={agent.name}
                          className="h-full w-full object-cover"
                          src={get_icon_avatar_src(agent.avatar) ?? undefined}
                        />
                      ) : (
                        get_initials(agent.name)
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold text-(--text-strong)">
                        {agent.name}
                      </p>
                      <p className="truncate text-[11px] text-(--text-soft)">
                        {t("room.add_member_dialog_hint")}
                      </p>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </>,
    document.body,
  );
}
