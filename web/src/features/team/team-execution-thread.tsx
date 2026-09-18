// INPUT: 已验证的本机任务绑定和在线 Agent 展示身份。
// OUTPUT: 原生 Room 消息、实时过程与权限响应驱动的共享 Thread。
// POS: 在线群到本机执行的窄适配，不创建会话、不重跑任务。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAgentConversation } from "@/hooks/agent/use-agent-conversation";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { TeamNodeJob } from "@/lib/api/conversation/team-node-api";
import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { getRoomThreadMessages } from "@/features/conversation/room/group/round/round-thread-model";
import { getRoomAgentRoundEntry, isAgentRoundActive } from "@/features/conversation/room/group/round/round-agent-model";
import { RoomThreadEmptyState } from "@/features/conversation/room/surface/room-thread-empty-state";
import { ComposerInteractionSurface } from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { RoomMobileOverlayFrame } from "@/features/conversation/room/surface/mobile/room-mobile-overlay-frame";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import type { RoomAgentExecutionState } from "@/types/agent/agent-conversation";
import type { TeamRoomBinding } from "@/lib/api/conversation/team-node-api";

export type TeamExecutionControls = Pick<ReturnType<typeof useAgentConversation>, "pending_permissions" | "stop_generation" | "send_permission_response" | "room_agent_execution_states" | "stopping_agent_round_ids">;

// 在线群打开就订阅本人已绑定的 Room，沿原生快照、重连与执行事件发现 Thread。
export function TeamExecutionObserver({binding, onChange, onControls}: {
  binding: TeamRoomBinding;
  onChange: (conversationId: string, states: RoomAgentExecutionState[]) => void;
  onControls?: (conversationId: string, controls: TeamExecutionControls | null) => void;
}) {
  const identity = useMemo(() => ({session_key: buildRoomSharedSessionKey(binding.conversation_id), agent_id: binding.local_agent_id, room_id: binding.room_id, conversation_id: binding.conversation_id, chat_type: "group" as const}), [binding.local_agent_id, binding.room_id, binding.conversation_id]);
  const conversation = useAgentConversation({identity});
  const {pending_permissions, stop_generation, send_permission_response, room_agent_execution_states, stopping_agent_round_ids} = conversation;
  useEffect(() => {
    onControls?.(binding.conversation_id, {pending_permissions, stop_generation, send_permission_response, room_agent_execution_states, stopping_agent_round_ids});
  }, [binding.conversation_id, onControls, pending_permissions, stop_generation, send_permission_response, room_agent_execution_states, stopping_agent_round_ids]);
  useEffect(() => () => onControls?.(binding.conversation_id, null), [binding.conversation_id, onControls]);
  const previous = useRef("");
  useEffect(() => {
    const states = conversation.room_agent_execution_states ?? [];
    const revision = JSON.stringify([
      conversation.ws_state, conversation.live_round_ids,
      (conversation.pending_agent_slots ?? []).map(({round_id, agent_id, status}) => [round_id, agent_id, status]),
      states.map(({round_id, agent_id, agent_round_id, status, phase}) => [round_id, agent_id, agent_round_id, status, phase]),
    ]);
    if (revision === previous.current) return;
    previous.current = revision;
    onChange(binding.conversation_id, states);
  }, [binding.conversation_id, conversation.room_agent_execution_states, conversation.pending_agent_slots, conversation.live_round_ids, conversation.ws_state, onChange]);
  return null;
}

export function TeamExecutionThread({job, name, avatar, compact, onClose, onOpenWorkspaceFile, hasFinalOutput = false, showInteraction = true}: {
  showInteraction?: boolean;
  hasFinalOutput?: boolean;
  job: TeamNodeJob; name: string; avatar?: string; compact: boolean; onClose: () => void; onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
}) {
  const { t } = useI18n();
  const [loadFailed, setLoadFailed] = useState(false);
  const [loading, setLoading] = useState(false);
  const sessionKey = buildRoomSharedSessionKey(job.conversation_id!);
  const identity = useMemo(() => ({session_key: sessionKey, agent_id: job.local_agent_id, room_id: job.room_id, conversation_id: job.conversation_id, chat_type: "group" as const}), [sessionKey, job.local_agent_id, job.room_id, job.conversation_id]);
  const conversation = useAgentConversation({identity});
  const load = conversation.load_round_window;
  const reload = useCallback(async () => {
    setLoading(true);
    try { setLoadFailed(!await load(job.round_id!)); }
    catch { setLoadFailed(true); }
    finally { setLoading(false); }
  }, [load, job.round_id]);
  const settled = hasFinalOutput || job.state === "draining" || job.state === "completed" || job.state === "failed" || job.state === "cancelled";
  // 完成信号同时触发持久历史补读，修复断线或订阅晚于终态事件的情况。
  useEffect(() => { void reload(); }, [reload, settled]);
  const roundMessages = conversation.messages.filter((item) => item.round_id === job.round_id);
  const entry = getRoomAgentRoundEntry(roundMessages, job.local_agent_id!,
    (conversation.pending_agent_slots ?? []).filter((item) => item.round_id === job.round_id), undefined,
    (conversation.room_agent_execution_states ?? []).filter((item) => item.round_id === job.round_id));
  const isExecuting = !settled && Boolean(entry ? isAgentRoundActive(entry.status) : job.state === "running" || job.state === "ready");
  const messages = getRoomThreadMessages(roundMessages, job.local_agent_id!);
  const permissions = conversation.pending_permissions.filter((item) => item.round_id === job.round_id && item.agent_id === job.local_agent_id);
  const panel = <ConversationThreadPanel agentId={job.local_agent_id!} agentName={name} agentAvatar={avatar}
    headerSubtitle={null}
    roundId={job.round_id!} messages={messages} sessionKey={sessionKey} workspaceAgentId={job.local_agent_id}
    pendingPermissions={permissions}
    onOpenWorkspaceFile={onOpenWorkspaceFile}
    footer={<>
      {loadFailed ? <div role="alert" className="p-3"><UiButton disabled={loading} onClick={() => { void reload(); }} variant="surface">{t("state.retry")}</UiButton></div> : null}
      {showInteraction && permissions.length ? <ComposerInteractionSurface permissions={permissions} onResponse={conversation.send_permission_response} fallbackAgentId={job.local_agent_id} agentNameMap={{[job.local_agent_id!]: name}} /> : null}
    </>}
    onStopMessage={(id) => {
      const message = messages.find((item) => item.message_id === id);
      if (message?.agent_round_id) conversation.stop_generation(message.agent_round_id);
    }}
    onPermissionResponse={conversation.send_permission_response} onClose={onClose}
    layout={compact ? "mobile" : "desktop"} presentation="inspector"
    isLoading={isExecuting}
    unresolvedToolStatus={entry?.status === "cancelled" || job.state === "cancelled" ? "stopped" : job.state === "failed" || entry?.status === "error" ? "error" : undefined}
    emptyContent={<RoomThreadEmptyState isLoading={isExecuting || loading} />} />;
  return compact ? <RoomMobileOverlayFrame label={`${name} Thread`} onClose={onClose}>{panel}</RoomMobileOverlayFrame> : panel;
}
