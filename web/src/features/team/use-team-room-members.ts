// INPUT: 在线 Room ID、成员快照与幂等治理命令。
// OUTPUT: 可重载的成员管理状态，以及邀请、角色、移除、撤销和群主移交动作。
// POS: Team Room 成员治理资源；新命令使用当前版本，未知结果保留原始版本与幂等键。

import { useCallback, useEffect, useRef, useState } from "react";
import { publishControlAgentApi } from "@/lib/api/account/control-api";
import { isTeamCommandUnapplied } from "./team-command-outcome";
import type { LauncherAgentSummary } from "@/types/app/launcher";
import { assertAuthOwnerScopeGenerationCurrent, captureAuthOwnerScopeGeneration, isAuthOwnerScopeGenerationCurrent } from "@/shared/auth/auth-owner-generation";

import {
	addTeamRoomAgent,
  getTeamRoom,
  inviteTeamRoomMember,
	removeTeamRoomAgent,
  revokeTeamRoomInvitation,
  transferTeamRoomOwnership,
  updateTeamRoomMember,
  updateTeamRoomSettings,
	updateTeamRoomCoordinator,
	updateTeamRoomAgent,
  type TeamRoomDetails,
} from "@/lib/api/conversation/team-api";

export function useTeamRoomMembers(roomId: string | null, open: boolean, onChanged?: (details: TeamRoomDetails) => void) {
  const [details, setDetails] = useState<TeamRoomDetails | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const pending = useRef<{ key: string; version: number; id: string } | null>(null);
  const busyRef = useRef(false);
  const onChangedRef = useRef(onChanged);
  onChangedRef.current = onChanged;

  const load = useCallback(async (signal?: AbortSignal) => {
    if (!roomId) return;
    const generation = captureAuthOwnerScopeGeneration();
    setLoading(true);
    try {
      const next = await getTeamRoom(roomId, signal);
      if (signal?.aborted || !isAuthOwnerScopeGenerationCurrent(generation)) return;
      setDetails(next);
      onChangedRef.current?.(next);
    } finally {
      if (!signal?.aborted && isAuthOwnerScopeGenerationCurrent(generation)) setLoading(false);
    }
  }, [roomId]);

  useEffect(() => {
    if (!open || !roomId) {
      setDetails(null);
      setFailed(false);
      return;
    }
    const controller = new AbortController();
    void load(controller.signal).catch(() => {
      if (!controller.signal.aborted) setFailed(true);
    });
    return () => controller.abort();
  }, [load, open, roomId]);

  const mutate = useCallback(async (
    action: string,
    target: string,
    apply: (version: number, commandId: string) => Promise<unknown>,
	version?: number,
  ) => {
    if (!details || busyRef.current) return false;
    const generation = captureAuthOwnerScopeGeneration();
    const key = `${action}:${target}`;
    const retrying = pending.current !== null;
    if (pending.current && pending.current.key !== key) { setFailed(true); return false; }
    const command = pending.current ?? { key, version: version ?? details.room.membership_version, id: crypto.randomUUID() };
    pending.current = command;
    busyRef.current = true;
    setBusy(true);
    setFailed(false);
    try {
      await apply(command.version, command.id);
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      pending.current = null;
      // 写入已确认，后续读取失败不能把它变回未知写入或诱导再次提交。
      await load().catch(() => setFailed(true));
      return true;
    } catch (cause) {
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      if (isTeamCommandUnapplied(cause, retrying)) pending.current = null;
      setFailed(true);
      await load().catch(() => undefined);
      return false;
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }, [details, load]);

  return {
	addAgent: (agent: LauncherAgentSummary) => mutate("add-agent", agent.id, async (version, commandId) => {
      const generation = captureAuthOwnerScopeGeneration();
      const published = await publishControlAgentApi(agent.id, agent);
      assertAuthOwnerScopeGenerationCurrent(generation);
      return addTeamRoomAgent(roomId ?? "", published.agent_id, version, commandId);
    }),
    busy,
    details,
    failed,
    loading,
    hasPendingCommand: pending.current !== null,
    settings: (change: { name?: string; avatar?: string; dissolve?: boolean }) => mutate("settings", JSON.stringify(change), (version, commandId) =>
      updateTeamRoomSettings(roomId ?? "", change, version, commandId), details?.room.configuration_version),
    retry: () => { setFailed(false); void load().catch(() => setFailed(true)); },
    invite: (userId: string) => mutate("invite", userId, (version, commandId) =>
      inviteTeamRoomMember(roomId ?? "", userId, version, commandId)),
    remove: (userId: string) => mutate("remove", userId, (version, commandId) =>
      updateTeamRoomMember(roomId ?? "", userId, version, { remove: true }, commandId)),
	removeAgent: (agentId: string) => mutate("remove-agent", agentId, (version, commandId) =>
	  removeTeamRoomAgent(roomId ?? "", agentId, version, commandId)),
	setCoordinator: (agentId: string) => mutate("coordinator", agentId, (version, commandId) =>
	  updateTeamRoomCoordinator(roomId ?? "", agentId, version, commandId), details?.room.configuration_version),
	setHostAutoReply: (enabled: boolean) => mutate("host-auto-reply", String(enabled), (version, commandId) =>
	  updateTeamRoomSettings(roomId ?? "", { host_auto_reply_enabled: enabled }, version, commandId), details?.room.configuration_version),
	setAgentPaused: (agentId: string, paused: boolean) => mutate(paused ? "pause-agent" : "resume-agent", agentId, (version, commandId) =>
	  updateTeamRoomAgent(roomId ?? "", agentId, paused, version, commandId)),
    revoke: (userId: string) => mutate("revoke", userId, (version, commandId) =>
      revokeTeamRoomInvitation(roomId ?? "", userId, version, commandId)),
    setRole: (userId: string, role: "admin" | "member") => mutate(`role:${role}`, userId, (version, commandId) =>
      updateTeamRoomMember(roomId ?? "", userId, version, { role }, commandId)),
    transfer: (userId: string) => mutate("transfer", userId, (version, commandId) =>
      transferTeamRoomOwnership(roomId ?? "", userId, version, commandId)),
  };
}
