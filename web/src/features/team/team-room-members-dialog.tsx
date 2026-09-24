// INPUT: 在线 Room、当前真人身份、Organization 目录与成员治理资源。
// OUTPUT: 邀请、角色、撤销、移除和真人群主移交弹窗。
// POS: Team Room Header 的成员管理表面；不解释 Relay 权限。

import { useEffect, useId, useMemo, useRef, useState } from "react";

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
import { MoreHorizontal, Plus, Trash2, Pause, Play, Crown } from "lucide-react";
import { UiActionMenu, type UiActionMenuItem } from "@/shared/ui/menu/action-menu";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
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
import { RoomDialogColumns, RoomIdentityFields, RoomMemberDirectory } from "@/features/conversation/room/members/room-dialog-layout";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { useTeamRoomMembers } from "./use-team-room-members";
import { useAuth } from "@/shared/auth/auth-context";

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
  const { status } = useAuth();
  const selfAvatar = (status?.control_user_id ?? status?.user_id) === currentUserId ? status?.avatar : undefined;
  const titleId = useId();
  const resource = useTeamRoomMembers(roomId, open, onChanged);
  const [settings, setSettings] = useState<{name: string; avatar: string} | null>(null);
  const [departure, setDeparture] = useState<"leave" | "dissolve" | null>(null);
  const [memberType, setMemberType] = useState<"users" | "agents">("users");
  const [query, setQuery] = useState("");
  const [directoryFailed, setDirectoryFailed] = useState(false);
  const [directoryLoading, setDirectoryLoading] = useState(true);
  const [directoryReload, setDirectoryReload] = useState(0);
	const [ownedAgents, setOwnedAgents] = useState<ControlAgent[]>([]);
	const [agentDirectory, setAgentDirectory] = useState<ControlAgentDirectoryEntry[]>([]);
	useEffect(() => {
		if (!open) return;
		let cancelled = false;
    setDirectoryFailed(false);
    setDirectoryLoading(true);
		void Promise.all([listControlAgentsApi(), listControlAgentDirectoryApi()])
			.then(([owned, directory]) => { if (!cancelled) { setOwnedAgents(owned); setAgentDirectory(directory); } })
			.catch(() => { if (!cancelled) { setDirectoryFailed(true); } })
      .finally(() => { if (!cancelled) setDirectoryLoading(false); });
		return () => { cancelled = true; };
	}, [open, directoryReload, resource.details?.room.membership_version]);
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

  const matches = (name: string) => name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase());
  const visibleMembers = members.filter((member) => matches(memberName(member.member_id)));
  const visibleCandidates = canManage ? candidates.filter((member) => matches(member.display_name || member.username)) : [];
  const visibleAgents = agentMembers.filter((member) => matches(onlineAgentNames.get(member.member_id) ?? member.member_id));
  const visibleAgentCandidates = current?.state === "active" && !directoryLoading && !directoryFailed
    ? agentCandidates.filter((agent) => matches(agent.name)) : [];
  const close = () => { if (!resource.busy) onClose(); };
  const saveSettings = () => { if (settings?.name.trim() && !resource.busy) void resource.settings(settings).then((ok) => { if (ok) setSettings(null); }); };
  const directoryCount = memberType === "users" ? visibleMembers.length + visibleCandidates.length : visibleAgents.length + visibleAgentCandidates.length;
  return (
    <>
      <UiDialogPortal>
        <UiDialogBackdrop labelledBy={titleId} onClose={close}>
          <UiDialogShell size="xl" viewport="adaptiveMax">
            <UiDialogHeader appearance="plain" onClose={close} title={t("room.settings_title")} titleId={titleId} />
            <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-5 px-5" scrollable>
              <RoomDialogColumns settings={<>
                {resource.details ? <>
                  <RoomIdentityFields avatar={settings?.avatar ?? resource.details.room.avatar}
                    name={settings?.name ?? resource.details.room.name} fallbackTitle={t("room.settings_title")}
                    disabled={!canChangeRoles || resource.busy || resource.hasPendingCommand}
                    onAvatarChange={(avatar) => setSettings({ name: settings?.name ?? resource.details!.room.name, avatar })}
                    onNameChange={(name) => setSettings({ name, avatar: settings?.avatar ?? resource.details!.room.avatar })}
                    onSubmit={canChangeRoles ? saveSettings : undefined} />
                  <div className="grid gap-2 border-t divider-subtle pt-3">
                    <span className="dialog-label">{t("team.coordinator_agent")}</span>
                    <UiSelectMenu ariaLabel={t("team.coordinator_agent")} disabled={!canChangeRoles || resource.busy}
                      options={[{ label: t("room.host_unset"), value: "" }, ...agentMembers.filter((member) => !member.agent_paused).map((member) => ({ label: onlineAgentNames.get(member.member_id) ?? member.member_id, value: member.member_id }))]}
                      onChange={(value) => { void resource.setCoordinator(value); }} size="sm" surface="dialog"
                      value={resource.details.room.coordinator_agent_id ?? ""} />
                    <label className="flex items-center gap-2 text-xs font-medium text-(--text-default)">
                      <UiCheckbox
                        checked={resource.details.room.host_auto_reply_enabled}
                        disabled={!canChangeRoles || resource.busy || !resource.details.room.coordinator_agent_id}
                        checkboxSize="small"
                        onChange={(event) => { void resource.setHostAutoReply(event.target.checked); }}
                      />
                      <span>{t("room.host_auto_reply_label")}</span>
                    </label>
                  </div>
                </> : null}
              </>}>
                <RoomMemberDirectory query={query} onQueryChange={setQuery}
                  searchLabel={t(memberType === "users" ? "room.search_user_placeholder" : "room.search_agent_placeholder")}
                  header={<UiSegmentedControl className="w-full" density="compact" stretch title={t("room.member_type")}
                    options={[{ label: t("room.people_count", { count: members.length }), value: "users" }, { label: t("room.agents_count", { count: agentMembers.length }), value: "agents" }]}
                    onChange={(value) => { setMemberType(value); setQuery(""); }} value={memberType} />}>
                  {memberType === "users" ? <>
                    {visibleMembers.map((member) => {
                      const name = memberName(member.member_id);
                      const editable = member.state === "active" && member.role !== "owner" && canChangeRoles;
                      const removable = member.state === "active" && member.role !== "owner" && canManage && !(current?.role === "admin" && member.role === "admin");
                      return <UiListRow key={member.member_id} className="max-sm:grid max-sm:grid-cols-[28px_minmax(0,1fr)_auto]" density="dense" title={name}
                        leading={<UiAgentAvatar avatar={member.member_id === currentUserId ? selfAvatar : directory.find((entry) => entry.user_id === member.member_id)?.avatar} name={name} size="sm" />}
                        right={editable ? <UiSelectMenu ariaLabel={t("team.member_role", { name })} className="w-24 shrink-0 max-sm:col-span-2 max-sm:col-start-2 max-sm:row-start-2 max-sm:w-full" disabled={resource.busy}
                          onChange={(value) => { void resource.setRole(member.member_id, value as "admin" | "member"); }}
                          options={[{ label: t("team.role_member"), value: "member" }, { label: t("team.role_admin"), value: "admin" }]}
                          size="sm" surface="plain" value={member.role} /> : <UiBadge size="xs" tone={member.state === "invited" ? "warning" : "default"}>{memberRole(member)}</UiBadge>}
                        actions={member.state === "invited" && canManage ? <UiIconButton aria-label={`${t("team.revoke")}: ${name}`} disabled={resource.busy} onClick={() => { void resource.revoke(member.member_id); }} size="xs" variant="ghost"><Trash2 className="h-3.5 w-3.5" /></UiIconButton>
                          : removable ? <TeamMemberActions name={name} disabled={resource.busy} items={[
                            { value: "remove", label: t("team.remove"), tone: "danger" },
                            ...(canChangeRoles ? [{ value: "transfer", label: t("team.transfer_owner") }] : []),
                          ]} onSelect={(value) => { if (value === "transfer") setTransferTarget(member); else void resource.remove(member.member_id); }} /> : null} />;
                    })}
                    {visibleCandidates.map((member) => <UiListRow key={member.user_id} density="dense" title={member.display_name || member.username}
                      leading={<UiAgentAvatar avatar={member.avatar} name={member.display_name || member.username} size="sm" />}
                      actions={<UiIconButton aria-label={`${t("team.invite")}: ${member.display_name || member.username}`} disabled={resource.busy} size="xs" variant="ghost" onClick={() => { void resource.invite(member.user_id); }}><Plus className="h-3.5 w-3.5" /></UiIconButton>} />)}
                  </> : <>
                    {visibleAgents.map((member) => {
                      const name = onlineAgentNames.get(member.member_id) ?? member.member_id;
                      const isOwner = member.agent_owner_user_id === currentUserId;
                      const ParticipationIcon = member.agent_paused ? Play : Pause;
                      return <UiListRow key={member.member_id} density="dense" title={name}
                        leading={<UiAgentAvatar avatar={agentDirectory.find((agent) => agent.agent_id === member.member_id)?.avatar} name={name} size="sm" />}
                        right={member.agent_paused && !isOwner ? <UiBadge size="xs" tone="warning">{t("team.agent_paused")}</UiBadge> : resource.details?.room.coordinator_agent_id === member.member_id ? <Crown aria-label={t("team.coordinator_agent")} role="img" className="h-4 w-4 shrink-0 text-(--accent)" /> : null}
                        actions={<>
                          {isOwner ? <UiChoiceButton active={member.agent_paused} aria-label={t(member.agent_paused ? "room.resume_member" : "room.pause_member", { name })} choiceSize="xs" disabled={resource.busy}
                            onClick={() => { void resource.setAgentPaused(member.member_id, !member.agent_paused); }} tone="neutral"><ParticipationIcon className="h-3 w-3" /><span className="max-sm:hidden">{t(member.agent_paused ? "room.resume_participation" : "room.pause_participation")}</span></UiChoiceButton> : null}
                          {isOwner || canManage ? <UiIconButton aria-label={`${t("team.remove")}: ${name}`} disabled={resource.busy} size="xs" variant="ghost" onClick={() => { void resource.removeAgent(member.member_id); }}><Trash2 className="h-3.5 w-3.5" /></UiIconButton> : null}
                        </>} />;
                    })}
                    {visibleAgentCandidates.map((agent) => <UiListRow key={agent.id} density="dense" title={agent.name}
                      leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />}
                      actions={<UiIconButton aria-label={`${t("team.add_agent")}: ${agent.name}`} disabled={resource.busy || directoryFailed} size="xs" variant="ghost" onClick={() => { void resource.addAgent(agent); }}><Plus className="h-3.5 w-3.5" /></UiIconButton>} />)}
                  </>}
                  {directoryCount === 0 && !resource.loading ? <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t(memberType === "users" ? "room.people_empty" : "room.agents_empty")}</p> : null}
                  {resource.loading && !resource.details ? <p role="status">{t("team.members_loading")}</p> : null}
                </RoomMemberDirectory>
              </RoomDialogColumns>
              {resource.failed || directoryFailed ? <UiInlineNotice message={t("team.members_action_failed")} tone="danger" /> : null}
            </UiDialogBody>
            <UiDialogFooter appearance="plain">
              {current?.state === "active" ? <UiButton className="mr-auto" disabled={resource.busy} size="sm" tone="danger" variant="text"
                onClick={() => setDeparture(canChangeRoles ? "dissolve" : "leave")}>{t(canChangeRoles ? "team.dissolve" : "team.leave")}</UiButton> : null}
              {resource.failed || directoryFailed || resource.hasPendingCommand ? <UiButton disabled={resource.busy || resource.loading} size="sm" onClick={() => { resource.retry(); setDirectoryReload((value) => value + 1); }} variant="text">{t("common.refresh")}</UiButton> : null}
              {canChangeRoles ? <UiButton disabled={resource.busy || !settings?.name.trim()} onClick={saveSettings} size="sm" variant="solid">{t("common.save")}</UiButton> : null}
              <UiButton disabled={resource.busy} onClick={onClose} size="sm" variant="surface">{t("common.close")}</UiButton>
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


function TeamMemberActions({ name, disabled, items, onSelect }: { name: string; disabled: boolean; items: UiActionMenuItem[]; onSelect: (value: string) => void }) {
  const { t } = useI18n();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  if (items.length === 1) return <UiIconButton aria-label={`${items[0].label}: ${name}`} disabled={disabled} size="xs" variant="ghost" onClick={() => onSelect(items[0].value)}><Trash2 className="h-3.5 w-3.5" /></UiIconButton>;
  return <>
    <UiIconButton aria-label={`${t("common.more_actions")}: ${name}`} aria-haspopup="menu" aria-expanded={open} disabled={disabled} ref={anchorRef} size="xs" variant="ghost" onClick={() => setOpen(!open)}><MoreHorizontal className="h-3.5 w-3.5" /></UiIconButton>
    <UiActionMenu align="end" anchorRef={anchorRef} ariaLabel={name} isOpen={open && !disabled} items={items} onClose={() => setOpen(false)} onSelect={(value) => { setOpen(false); onSelect(value); }} />
  </>;
}
