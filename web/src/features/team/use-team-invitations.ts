// INPUT: 远程登录状态、Relay invitation 目录与接受/拒绝命令。
// OUTPUT: 当前用户的待处理邀请、单飞处理状态和刷新动作。
// POS: 聊天目录在线邀请资源；接受成功后通知 Room 目录刷新。

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

import {
  listTeamInvitations,
  resolveTeamRoomInvitation,
  transferTeamRoomOwnership,
  type TeamRoomRecovery,
  type TeamRoomInvitation,
} from "@/lib/api/conversation/team-api";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { isRemoteAccountAuthenticated, useAuth } from "@/shared/auth/auth-context";
import { useTeamRefresh } from "./use-team-refresh";
import { isTeamCommandUnapplied } from "./team-command-outcome";

export function useTeamInvitations(onAccepted: () => void) {
  const { status } = useAuth();
  const enabled = isRemoteAccountAuthenticated(status);
  const generation = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const [invitations, setInvitations] = useState<TeamRoomInvitation[]>([]);
  const [recoveryRooms, setRecoveryRooms] = useState<TeamRoomRecovery[]>([]);
  const [busyRoomId, setBusyRoomId] = useState<string | null>(null);
  const [errorRoomId, setErrorRoomId] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [loading, setLoading] = useState(false);
  const busyRef = useRef(false);
  const commands = useRef(new Map<string, { id: string; version: number }>());
  const readVersion = useRef(0);

  useEffect(() => {
    setInvitations([]);
    setRecoveryRooms([]);
    setFailed(false);
    setErrorRoomId(null);
    setBusyRoomId(null);
    commands.current.clear();
    busyRef.current = false;
  }, [enabled, generation]);
  const refresh = useTeamRefresh(enabled ? String(generation) : null, async (signal) => {
    const version = readVersion.current;
    setLoading(true);
    try {
      const result = await listTeamInvitations(signal);
      if (!signal.aborted && isAuthOwnerScopeGenerationCurrent(generation) && version === readVersion.current) {
        setInvitations(result.invitations);
        setRecoveryRooms(result.recovery_rooms ?? []);
        setFailed(false);
      }
    } catch {
      if (!signal.aborted && isAuthOwnerScopeGenerationCurrent(generation)) setFailed(true);
    } finally {
      if (!signal.aborted && isAuthOwnerScopeGenerationCurrent(generation)) setLoading(false);
    }
  });

  const resolve = useCallback(async (invitation: TeamRoomInvitation, resolution: "accept" | "reject") => {
    if (!enabled || busyRef.current) return;
    busyRef.current = true;
    const commandKey = `${invitation.room.id}:${resolution}`;
    const retrying = commands.current.has(commandKey);
    const command = commands.current.get(commandKey) ?? { id: crypto.randomUUID(), version: invitation.room.membership_version };
    commands.current.set(commandKey, command);
    setBusyRoomId(invitation.room.id);
    setErrorRoomId(null);
    try {
      await resolveTeamRoomInvitation(
        invitation.room.id,
        command.version,
        resolution,
        command.id,
      );
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return;
      commands.current.delete(commandKey);
      readVersion.current += 1;
      setInvitations((items) => items.filter((item) => item.room.id !== invitation.room.id));
      if (resolution === "accept") onAccepted();
    } catch (cause) {
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return;
      if (isTeamCommandUnapplied(cause, retrying)) commands.current.delete(commandKey);
      // 接受可能已经提交；邀请目录和已加入目录必须一起对账。
      if (resolution === "accept") onAccepted();
      setErrorRoomId(invitation.room.id);
      refresh();
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(generation)) { busyRef.current = false; setBusyRoomId(null); }
    }
  }, [enabled, generation, onAccepted, refresh]);

  const recover = async (room: TeamRoomRecovery) => {
    const userId = status?.control_user_id ?? status?.user_id;
    if (!enabled || !userId || busyRef.current) return false;
    const key = `${room.id}:recover`;
    const retrying = commands.current.has(key);
    const command = commands.current.get(key) ?? {id: crypto.randomUUID(), version: room.membership_version};
    commands.current.set(key, command);
    busyRef.current = true;
    setBusyRoomId(room.id);
    setErrorRoomId(null);
    try {
      await transferTeamRoomOwnership(room.id, userId, command.version, command.id, true);
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      commands.current.delete(key);
      readVersion.current += 1;
      setRecoveryRooms((items) => items.filter((item) => item.id !== room.id));
      onAccepted();
      return true;
    } catch (cause) {
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      if (isTeamCommandUnapplied(cause, retrying)) commands.current.delete(key);
      setErrorRoomId(room.id);
      onAccepted();
      refresh();
      return false;
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(generation)) { busyRef.current = false; setBusyRoomId(null); }
    }
  };

  return { busyRoomId, enabled, errorRoomId, failed, loading, invitations, recoveryRooms, recover, refresh, resolve };
}
