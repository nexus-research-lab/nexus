// INPUT: 在线 Room、当前真人身份、Organization 目录与成员治理资源。
// OUTPUT: 邀请、角色、撤销、移除和真人群主移交弹窗。
// POS: Team Room Header 的成员管理表面；不解释 Relay 权限。

import { useEffect, useId, useMemo, useState } from "react";

import {
	listControlAgentsApi,
	listControlAgentDirectoryApi,
	type ControlAgent,
	type ControlAgentDirectoryEntry,
	type ControlMemberDirectoryEntry,
} from "@/lib/api/account/control-api";
import type { LauncherAgentSummary } from "@/types/app/launcher";
import type { TeamRoomMember, TeamRoomDetails } from "@/lib/api/conversation/team-api";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { UiInput } from "@/shared/ui/form/form-control";
import { RoomAvatarPicker } from "@/features/conversation/room/members/room-avatar-picker";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { useTeamRoomMembers } from "./use-team-room-members";

export function TeamRoomMembersDialog({
	agents,
  currentUserId,
  directory,
  onClose,
  onChanged,
  onDeparted,
  open,
  roomId,
}: {
	agents: LauncherAgentSummary[];
  currentUserId: string;
  directory: ControlMemberDirectoryEntry[];
  onClose: () => void;
  onChanged: (details: TeamRoomDetails) => void;
  onDeparted: () => void;
  open: boolean;
  roomId: string | null;
}) {
  const { t } = useI18n();
  const titleId = useId();
  const resource = useTeamRoomMembers(roomId, open, onChanged);
  const [settings, setSettings] = useState<{name: string; avatar: string} | null>(null);
  const [departure, setDeparture] = useState<"leave" | "dissolve" | null>(null);
	const [invitee, setInvitee] = useState("");
	const [agentCandidate, setAgentCandidate] = useState("");
	const [ownedAgents, setOwnedAgents] = useState<ControlAgent[]>([]);
	const [agentDirectory, setAgentDirectory] = useState<ControlAgentDirectoryEntry[]>([]);
	useEffect(() => {
		if (!open) return;
		let cancelled = false;
		void Promise.all([listControlAgentsApi(), listControlAgentDirectoryApi()])
			.then(([owned, directory]) => { if (!cancelled) { setOwnedAgents(owned); setAgentDirectory(directory); } })
			.catch(() => { if (!cancelled) { setOwnedAgents([]); setAgentDirectory([]); } });
		return () => { cancelled = true; };
	}, [open, resource.details?.room.membership_version]);
  const [transferTarget, setTransferTarget] = useState<TeamRoomMember | null>(null);
  const members = resource.details?.members.filter((member) => member.member_type === "user" && (member.state === "active" || member.state === "invited")) ?? [];
  const current = members.find((member) => member.member_id === currentUserId);
  const canManage = current?.role === "owner" || current?.role === "admin";
  const canChangeRoles = current?.role === "owner";
  const names = useMemo(() => new Map(directory.map((member) => [member.user_id, member.display_name || member.username])), [directory]);
  const existing = new Set(members.map((member) => member.member_id));
	const candidates = directory.filter((member) => !existing.has(member.user_id));
	const agentMembers = resource.details?.members.filter((member) => member.member_type === "agent" && member.state === "active") ?? [];
	const existingAgentIDs = new Set(agentMembers.map((member) => member.member_id));
	const onlineAgentBySourceID = new Map(ownedAgents.map((agent) => [agent.source_agent_id, agent]));
	const agentCandidates = agents.filter((agent) => {
		const online = onlineAgentBySourceID.get(agent.id);
		return !online || !existingAgentIDs.has(online.agent_id);
	});
	const onlineAgentNames = new Map(agentDirectory.map((agent) => [agent.agent_id, agent.name]));

  if (!open) return null;
  const memberName = (userId: string) => userId === currentUserId ? t("team.you") : names.get(userId) ?? userId;
  const memberRole = (member: TeamRoomMember) => {
    if (member.state === "invited") return t("team.member_invited");
    if (member.role === "owner") return t("team.role_owner");
    if (member.role === "admin") return t("team.role_admin");
    return t("team.role_member");
  };

  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop labelledBy={titleId} onClose={() => { if (!resource.busy) onClose(); }}>
          <UiDialogShell size="md" viewport="adaptiveMax">
            <UiDialogHeader appearance="plain" onClose={() => { if (!resource.busy) onClose(); }} title={t("team.members_title")} titleId={titleId} />
            <UiDialogBody className="space-y-5 px-5" scrollable>
              {canChangeRoles && resource.details ? (
                <section className="space-y-2">
                  <h3 className={getUiTypographyClassName({role: "sectionTitle"})}>{t("room.settings_title")}</h3>
                  <div className="flex flex-wrap items-center gap-2">
                    <RoomAvatarPicker avatar={settings?.avatar ?? resource.details.room.avatar}
                      disabled={resource.busy || resource.hasPendingCommand} fallbackTitle={resource.details.room.name}
                      name={settings?.name ?? resource.details.room.name}
                      onChange={(avatar) => setSettings({name: settings?.name ?? resource.details!.room.name, avatar})} />
                    <UiInput aria-label={t("team.room_name")} className="min-w-0 flex-1" maxLength={64}
                      disabled={resource.busy || resource.hasPendingCommand}
                      value={settings?.name ?? resource.details.room.name}
                      onChange={(event) => setSettings({name: event.target.value, avatar: settings?.avatar ?? resource.details!.room.avatar})} />
                    <UiButton disabled={resource.busy || !settings?.name.trim()} size="sm" variant="solid"
                      onClick={() => { if (settings) void resource.settings(settings).then((ok) => { if (ok) setSettings(null); }); }}>{t("common.save")}</UiButton>
                  </div>
                  {resource.details.room.coordinator_agent_id ? <UiButton disabled={resource.busy} size="xs" variant="text"
                    onClick={() => { void resource.setCoordinator(""); }}>{t("team.clear_coordinator")}</UiButton> : null}
                </section>
              ) : null}
              {canManage ? (
			  <section className="space-y-2">
                  <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{t("team.invite_member")}</h3>
                  <div className="flex gap-2">
                    <UiSelectMenu
                      ariaLabel={t("team.invite_member")}
                      className="min-w-0 flex-1"
                      disabled={resource.busy || candidates.length === 0}
                      onChange={setInvitee}
                      options={candidates.map((member) => ({ label: member.display_name || member.username, value: member.user_id }))}
                      placeholder={t(candidates.length === 0 ? "team.no_invite_candidates" : "team.select_member")}
                      value={invitee}
                    />
                    <UiButton disabled={resource.busy || !invitee} onClick={() => { void resource.invite(invitee).then((ok) => { if (ok) setInvitee(""); }); }} size="sm" variant="solid">
                      {t("team.invite")}
                    </UiButton>
                  </div>
			  </section>
			  ) : null}

			  <section className="space-y-2">
				<h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{t("room.agents_count", { count: agentMembers.length })}</h3>
				<div className="flex gap-2">
				  <UiSelectMenu
					ariaLabel={t("room.search_agent_placeholder")}
					className="min-w-0 flex-1"
					disabled={resource.busy || agentCandidates.length === 0}
					onChange={setAgentCandidate}
					options={agentCandidates.map((agent) => ({ label: agent.name, value: agent.id }))}
					placeholder={t("team.select_agent")}
					value={agentCandidate}
				  />
				  <UiButton disabled={resource.busy || !agentCandidate} onClick={() => {
					const local = agents.find((agent) => agent.id === agentCandidate);
					if (!local) return;
					void resource.addAgent(local).then((ok) => { if (ok) setAgentCandidate(""); });
				  }} size="sm" variant="solid">{t("team.add_agent")}</UiButton>
				</div>
				<div className="divide-y divide-(--surface-panel-border) rounded-xl border border-(--surface-panel-border)">
				  {agentMembers.map((member) => (
					<div className="flex min-h-14 items-center gap-2 px-3 py-2" key={member.member_id}>
					  <span className="min-w-0 flex-1 truncate text-sm font-medium">{onlineAgentNames.get(member.member_id) ?? member.member_id}</span>
					  {member.agent_paused ? <UiBadge size="xs" tone="warning">{t("team.agent_paused")}</UiBadge> : null}
					  {resource.details?.room.coordinator_agent_id === member.member_id ? (
						<UiBadge size="xs">{t("team.coordinator_agent")}</UiBadge>
					  ) : canChangeRoles && !member.agent_paused ? (
						<UiButton disabled={resource.busy} onClick={() => { void resource.setCoordinator(member.member_id); }} size="xs" variant="text">{t("team.set_coordinator_agent")}</UiButton>
					  ) : null}
					  {member.agent_owner_user_id === currentUserId ? (
						<UiButton disabled={resource.busy} onClick={() => { void resource.setAgentPaused(member.member_id, !member.agent_paused); }} size="xs" variant="text">
						  {t(member.agent_paused ? "room.resume_participation" : "room.pause_participation")}
						</UiButton>
					  ) : null}
					  {(member.agent_owner_user_id === currentUserId || canManage) ? (
						<UiButton disabled={resource.busy} onClick={() => { void resource.removeAgent(member.member_id); }} size="xs" tone="danger" variant="text">{t("team.remove")}</UiButton>
					  ) : null}
					</div>
				  ))}
				</div>
			  </section>

              <section className="space-y-2">
                <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{t("team.members")}</h3>
                <div className="divide-y divide-(--surface-panel-border) rounded-xl border border-(--surface-panel-border)">
                  {members.map((member) => (
                    <div className="flex min-h-14 flex-wrap items-center gap-2 px-3 py-2" key={member.member_id}>
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">{memberName(member.member_id)}</span>
                      <UiBadge size="xs" tone={member.state === "invited" ? "warning" : "default"}>
                        {memberRole(member)}
                      </UiBadge>
                      {member.state === "invited" && canManage ? (
                        <UiButton disabled={resource.busy} onClick={() => { void resource.revoke(member.member_id); }} size="xs" tone="danger" variant="text">{t("team.revoke")}</UiButton>
                      ) : null}
                      {member.state === "active" && member.role !== "owner" && canChangeRoles ? (
                        <UiSelectMenu
                          ariaLabel={t("team.member_role", { name: memberName(member.member_id) })}
                          disabled={resource.busy}
                          onChange={(value) => { void resource.setRole(member.member_id, value as "admin" | "member"); }}
                          options={[
                            { label: t("team.role_member"), value: "member" },
                            { label: t("team.role_admin"), value: "admin" },
                          ]}
                          size="sm"
                          value={member.role}
                        />
                      ) : null}
                      {member.state === "active" && member.role !== "owner" && canManage && !(current?.role === "admin" && member.role === "admin") ? (
                        <UiButton disabled={resource.busy} onClick={() => { void resource.remove(member.member_id); }} size="xs" tone="danger" variant="text">{t("team.remove")}</UiButton>
                      ) : null}
                      {member.state === "active" && member.role !== "owner" && canChangeRoles ? (
                        <UiButton disabled={resource.busy} onClick={() => setTransferTarget(member)} size="xs" variant="text">{t("team.transfer_owner")}</UiButton>
                      ) : null}
                    </div>
                  ))}
                </div>
              </section>
              {resource.failed ? <UiInlineNotice message={t("team.members_action_failed")} tone="danger" /> : null}
              {resource.loading && !resource.details ? <p role="status">{t("team.members_loading")}</p> : null}
            </UiDialogBody>
            <UiDialogFooter appearance="plain">
              {current?.state === "active" ? <UiButton disabled={resource.busy} tone="danger" variant="text"
                onClick={() => setDeparture(canChangeRoles ? "dissolve" : "leave")}>{t(canChangeRoles ? "team.dissolve" : "team.leave")}</UiButton> : null}
              <UiButton disabled={resource.busy || resource.loading} onClick={resource.retry} variant="text">{t("common.refresh")}</UiButton>
              <UiButton disabled={resource.busy} onClick={onClose} variant="surface">{t("common.close")}</UiButton>
            </UiDialogFooter>
          </UiDialogShell>
        </UiDialogBackdrop>
      </UiDialogPortal>
      <ConfirmDialog
        busy={resource.busy}
        confirmText={t(departure === "dissolve" ? "team.dissolve" : "team.leave")}
        isOpen={departure !== null}
        message={t(departure === "dissolve" ? "team.dissolve_confirm" : "team.leave_confirm")}
        onCancel={() => setDeparture(null)}
        onConfirm={() => {
          const command = departure === "dissolve" ? resource.settings({dissolve: true}) : resource.remove(currentUserId);
          void command.then((ok) => { if (ok) { setDeparture(null); onClose(); onDeparted(); } });
        }}
        title={t(departure === "dissolve" ? "team.dissolve" : "team.leave")}
      />
      <ConfirmDialog
        busy={resource.busy}
        confirmText={t("team.transfer_owner")}
        isOpen={transferTarget !== null}
        message={t("team.transfer_owner_confirm", { name: memberName(transferTarget?.member_id ?? "") })}
        onCancel={() => setTransferTarget(null)}
        onConfirm={() => { if (transferTarget) void resource.transfer(transferTarget.member_id).then((ok) => { if (ok) setTransferTarget(null); }); }}
        title={t("team.transfer_owner")}
      />
    </>
  );
}
