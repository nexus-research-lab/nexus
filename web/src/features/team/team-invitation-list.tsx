// INPUT: 当前真人的在线 Room 邀请与接受/拒绝动作。
// OUTPUT: 聊天目录中的紧凑邀请卡片。
// POS: 展示 pending 邀请及刷新入口，不拥有远程状态。

import { UiButton } from "@/shared/ui/button/button";
import { useState } from "react";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type { TeamRoomInvitation, TeamRoomRecovery } from "@/lib/api/conversation/team-api";

export function TeamInvitationList({
  busyRoomId,
  errorRoomId,
  invitations,
  failed,
  loading,
  onRefresh,
  onResolve,
  recoveryRooms,
  onRecover,
}: {
  busyRoomId: string | null;
  errorRoomId: string | null;
  invitations: TeamRoomInvitation[];
  failed: boolean;
  loading: boolean;
  onRefresh: () => void;
  onResolve: (invitation: TeamRoomInvitation, resolution: "accept" | "reject") => void;
  recoveryRooms: TeamRoomRecovery[];
  onRecover: (room: TeamRoomRecovery) => Promise<boolean>;
}) {
  const { t } = useI18n();
  const [recovery, setRecovery] = useState<TeamRoomRecovery | null>(null);
  return (
    <section aria-label={t("team.pending_invitations")} className="mx-2 mb-2 space-y-1.5">
      <div className="flex items-center justify-between">
      <p className={`px-1 ${getUiTypographyClassName({ role: "supporting", tone: "muted", weight: "medium" })}`}>
        {t("team.pending_invitations")}
      </p>
      <UiButton disabled={loading} aria-busy={loading} onClick={onRefresh} size="xs" variant="text">{t("common.refresh")}</UiButton>
      </div>
      {failed ? <p role="alert" className={getUiTypographyClassName({ role: "supporting", tone: "danger" })}>{t("team.invitation_failed")}</p> : null}
      {invitations.map((invitation) => {
        const busy = busyRoomId === invitation.room.id;
        return (
          <div className="surface-radius-md border border-(--surface-panel-border) bg-(--surface-panel-background) p-2.5" key={invitation.room.id}>
            <div className="flex items-center gap-2">
              <UiRoomAvatar avatar={invitation.room.avatar} roomId={invitation.room.id} size="sm" title={invitation.room.name} members={[]} />
              <span className="min-w-0 flex-1 truncate text-sm font-medium">{invitation.room.name}</span>
            </div>
            <div className="mt-2 flex justify-end gap-1.5">
              <UiButton disabled={busy} onClick={() => onResolve(invitation, "reject")} size="xs" variant="text">
                {t("team.invitation_reject")}
              </UiButton>
              <UiButton aria-busy={busy} disabled={busy} onClick={() => onResolve(invitation, "accept")} size="xs" variant="solid">
                {t("team.invitation_accept")}
              </UiButton>
            </div>
            {errorRoomId === invitation.room.id ? (
              <p className={`mt-1.5 ${getUiTypographyClassName({ role: "supporting", tone: "danger" })}`} role="alert">
                {t("team.invitation_failed")}
              </p>
            ) : null}
          </div>
        );
      })}
      {recoveryRooms.map((room) => <div className="flex flex-wrap items-center gap-2" key={room.id}>
        <span className="min-w-0 flex-1 truncate">{room.name}</span>
        <UiButton disabled={busyRoomId !== null} size="xs" variant="text" onClick={() => setRecovery(room)}>{t("team.recover_owner")}</UiButton>
        {errorRoomId === room.id ? <p role="alert">{t("team.invitation_failed")}</p> : null}
      </div>)}
      <ConfirmDialog isOpen={recovery !== null} busy={busyRoomId !== null}
        title={t("team.recover_owner")} message={t("team.recover_owner_confirm", {name: recovery?.name ?? ""})}
        confirmText={t("team.recover_owner")} onCancel={() => setRecovery(null)}
        onConfirm={() => { if (recovery) void onRecover(recovery).then((ok) => { if (ok) setRecovery(null); }); }} />
    </section>
  );
}
