"use client";

import { X } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { DIALOG_EMPTY_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";

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

  return (
    <div
      className="dialog-backdrop"
      role="dialog"
      aria-modal="true"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-lg">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title">{t("room.add_member_dialog_title")}</h3>
            <p className="dialog-subtitle">
              {t("room.add_member_dialog_subtitle")}
            </p>
          </div>
          <WorkspacePillButton
            aria-label={t("common.close")}
            density="compact"
            onClick={on_cancel}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
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
                    "surface-card flex w-full items-center gap-3 rounded-[20px] px-4 py-3 text-left transition-all duration-300 hover:-translate-y-0.5",
                  )}
                  onClick={() => on_select(agent.agent_id)}
                  type="button"
                >
                  <div className="chip-default flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-slate-900/82">
                    {agent.name.slice(0, 2).toUpperCase()}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-slate-900/88">
                      {agent.name}
                    </p>
                    <p className="truncate text-[11px] text-slate-700/52">
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
  );
}
