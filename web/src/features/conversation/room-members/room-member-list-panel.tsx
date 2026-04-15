/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：room-member-list-panel.tsx
# @Date   ：2026-04-12 15:32
# @Author ：leemysw
# 2026-04-12 15:32   Create
# =====================================================
*/

"use client";

import { createPortal } from "react-dom";
import { Plus, Trash2, Users, X } from "lucide-react";
import { useMemo, useState } from "react";

import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_SHELL_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { get_icon_avatar_src, get_initials } from "@/lib/utils";
import { Agent } from "@/types/agent";

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

  if (!is_open || typeof document === "undefined") {
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

  const dialog = (
    <>
      <div className="dialog-backdrop z-[9998]" onClick={on_close} role="dialog" aria-modal="true">
        <div
          className={DIALOG_SHELL_CLASS_NAME}
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
          onPointerMove={(event) => event.stopPropagation()}
          onPointerUp={(event) => event.stopPropagation()}
          data-modal-root="true"
        >
          <div className="dialog-header">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--text-strong) shadow-(--surface-avatar-shadow)">
                <Users className="h-4.5 w-4.5" />
              </div>
              <div className="min-w-0">
                <h3 className="dialog-title">{t("room.members_title")}</h3>
                <p className="dialog-subtitle">
                  {t("room.members_subtitle")}
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

              <div className="mt-4 space-y-2">
                {members.length > 0 ? (
                  members.map((member, index) => {
                    const avatar_src = get_icon_avatar_src(member.avatar);
                    const is_owner = index === 0;
                    const can_remove = !is_owner;

                    return (
                      <div
                        key={member.agent_id}
                        className="flex items-center gap-3 rounded-[16px] border border-(--divider-subtle-color) px-3.5 py-3"
                      >
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
                          {avatar_src ? (
                            <img
                              alt={member.name}
                              className="h-full w-full object-cover"
                              src={avatar_src}
                            />
                          ) : (
                            get_initials(member.name)
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <p className="truncate text-sm font-semibold text-(--text-strong)">
                              {member.name}
                            </p>
                            {is_owner ? (
                              <span className="inline-flex h-5 items-center rounded-full border border-(--divider-subtle-color) px-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
                                {t("room.member_owner")}
                              </span>
                            ) : null}
                          </div>
                          <p className="mt-1 text-xs text-(--text-muted)">
                            {is_owner ? t("room.member_owner_hint") : t("room.member_collaborator_hint")}
                          </p>
                        </div>
                        {can_remove ? (
                          <button
                            className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] text-(--icon-muted) transition duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] hover:text-(--destructive)"
                            disabled={is_removing === member.agent_id}
                            onClick={() => {
                              void handle_remove_member(member.agent_id);
                            }}
                            type="button"
                            title={t("common.remove")}
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        ) : (
                          <span className="w-8 shrink-0" />
                        )}
                      </div>
                    );
                  })
                ) : (
                  <div className="surface-inset flex items-center justify-center rounded-[16px] px-4 py-8 text-[13px] text-(--text-muted)">
                    {t("room.no_members")}
                  </div>
                )}
              </div>
            </section>

            {available_agents.length === 0 && removable_members.length === 0 ? (
              <div className="rounded-[18px] border border-(--divider-subtle-color) px-4 py-3 text-[12px] leading-6 text-(--text-muted)">
                {t("room.no_available_members")}
              </div>
            ) : null}
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
    </>
  );

  return createPortal(dialog, document.body);
}
