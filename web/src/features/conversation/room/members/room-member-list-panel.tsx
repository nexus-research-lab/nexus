"use client";

import { Plus, Trash2, Users } from "lucide-react";
import { useMemo, useState } from "react";

import {
  DIALOG_HEADER_ICON_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiBadge } from "@/shared/ui/badge";
import { UiIconButton } from "@/shared/ui/button";
import { UiListRow } from "@/shared/ui/list-row";
import { Agent } from "@/types/agent/agent";

import { RoomMemberPickerDialog } from "./room-member-picker-dialog";

interface RoomMemberListPanelProps {
  is_open: boolean;
  members: Agent[];
  available_agents: Agent[];
  on_add_member: (agent_id: string) => Promise<void>;
  on_remove_member: (agent_id: string) => Promise<void>;
  on_close: () => void;
}

export function RoomMemberListPanel({
  is_open,
  members,
  available_agents,
  on_add_member,
  on_remove_member,
  on_close,
}: RoomMemberListPanelProps) {
  const { t } = useI18n();
  const [is_picker_open, set_is_picker_open] = useState(false);
  const [is_removing, set_is_removing] = useState<string | null>(null);
  const removable_members = useMemo(() => members.slice(1), [members]);

  if (!is_open) {
    return null;
  }

  const handle_remove_member = async (agent_id: string) => {
    set_is_removing(agent_id);
    try {
      await on_remove_member(agent_id);
    } finally {
      set_is_removing(null);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9998]"
        labelled_by="room-members-dialog-title"
        on_close={on_close}
      >
        <UiDialogShell size="md">
          <UiDialogHeader>
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div className={DIALOG_HEADER_ICON_CLASS_NAME}>
                <Users className="h-4.5 w-4.5" />
              </div>
              <div className="min-w-0">
                <h3 className="dialog-title" id="room-members-dialog-title">{t("room.members_title")}</h3>
                <p className="dialog-subtitle">
                  {t("room.members_subtitle")}
                </p>
              </div>
            </div>
            <UiDialogCloseButton on_close={on_close} />
          </UiDialogHeader>

          <UiDialogBody class_name="space-y-4">
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-(--text-strong)">
                    {t("room.current_members")}
                  </p>
                </div>
                {available_agents.length > 0 ? (
                  <button
                    className={get_dialog_action_class_name("primary")}
                    onClick={() => set_is_picker_open(true)}
                    type="button"
                  >
                    <Plus className="h-4 w-4" />
                    {t("room.add_member")}
                  </button>
                ) : null}
              </div>

              <div className="space-y-1.5">
                {members.length > 0 ? (
                  members.map((member, index) => {
                    const is_owner = index === 0;
                    const can_remove = !is_owner;

                    return (
                      <UiListRow
                        key={member.agent_id}
                        class_name="min-h-[60px] px-2.5 py-2"
                        description={is_owner ? t("room.member_owner_hint") : t("room.member_collaborator_hint")}
                        leading={<UiAgentAvatar avatar={member.avatar} name={member.name} />}
                        meta={is_owner ? (
                          <UiBadge class_name="uppercase tracking-[0.14em]" size="xs">
                            {t("room.member_owner")}
                          </UiBadge>
                        ) : null}
                        right={can_remove ? (
                          <UiIconButton
                            disabled={is_removing === member.agent_id}
                            onClick={() => {
                              void handle_remove_member(member.agent_id);
                            }}
                            size="md"
                            title={t("common.remove")}
                            tone="danger"
                            type="button"
                            variant="ghost"
                          >
                            <Trash2 className="h-4 w-4" />
                          </UiIconButton>
                        ) : (
                          <span className="w-8 shrink-0" />
                        )}
                        title={member.name}
                      />
                    );
                  })
                ) : (
                  <div className="flex items-center justify-center rounded-[12px] px-4 py-8 text-[13px] text-(--text-muted)">
                    {t("room.no_members")}
                  </div>
                )}
              </div>
            </div>

            {available_agents.length === 0 && removable_members.length === 0 ? (
              <div className="px-1 text-[12px] leading-6 text-(--text-muted)">
                {t("room.no_available_members")}
              </div>
            ) : null}
          </UiDialogBody>

          <UiDialogFooter>
            <button
              className={get_dialog_action_class_name("default")}
              onClick={on_close}
              type="button"
            >
              {t("common.close")}
            </button>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>

      <RoomMemberPickerDialog
        agents={available_agents}
        is_open={is_picker_open}
        on_cancel={() => set_is_picker_open(false)}
        on_select={(agent_id) => {
          void on_add_member(agent_id).then(() => {
            set_is_picker_open(false);
          });
        }}
      />
    </UiDialogPortal>
  );
}
