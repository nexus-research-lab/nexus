"use client";

import { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import {
  DIALOG_EMPTY_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
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

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        labelled_by="room-member-picker-dialog-title"
        on_close={on_cancel}
      >
        <UiDialogShell size="md">
          <UiDialogHeader>
            <div className="min-w-0 flex-1">
              <h3 className="dialog-title" id="room-member-picker-dialog-title">
                {t("room.add_member_dialog_title")}
              </h3>
              <p className="dialog-subtitle">
                {t("room.add_member_dialog_subtitle")}
              </p>
            </div>
            <UiDialogCloseButton on_close={on_cancel} />
          </UiDialogHeader>

          <UiDialogBody>
            {agents.length === 0 ? (
              <div className={DIALOG_EMPTY_CLASS_NAME}>
                {t("room.no_available_members")}
              </div>
            ) : (
              <div className="soft-scrollbar max-h-[360px] space-y-1.5 overflow-y-auto pr-1">
                {agents.map((agent) => (
                  <UiListRow
                    class_name="min-h-[60px] px-2.5 py-2"
                    description={t("room.add_member_dialog_hint")}
                    key={agent.agent_id}
                    leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} />}
                    on_click={() => on_select(agent.agent_id)}
                    title={agent.name}
                  />
                ))}
              </div>
            )}
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
