"use client";

import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import {
  DIALOG_EMPTY_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import { UiListRow } from "@/shared/ui/list-row";

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
                  <UiListRow
                    class_name="min-h-[64px] border border-(--divider-subtle-color) px-4 py-3"
                    description={t("room.add_member_dialog_hint")}
                    key={agent.agent_id}
                    leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} />}
                    on_click={() => on_select(agent.agent_id)}
                    title={agent.name}
                  />
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
