// INPUT: 已验证的本机任务绑定和在线 Agent 展示身份。
// OUTPUT: 原生 Room 消息、实时过程与权限响应驱动的共享 Thread。
// POS: 在线群到本机执行的窄适配，不创建会话、不重跑任务。
import { useCallback, useEffect, useMemo, useState } from "react";
import { useAgentConversation } from "@/hooks/agent/use-agent-conversation";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { TeamNodeJob } from "@/lib/api/conversation/team-node-api";
import { ConversationThreadPanel } from "@/features/conversation/shared/thread/conversation-thread-panel";
import { getRoomThreadMessages } from "@/features/conversation/room/group/round/round-thread-model";
import { RoomThreadEmptyState } from "@/features/conversation/room/surface/room-thread-empty-state";
import { ComposerInteractionSurface } from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { RoomMobileOverlayFrame } from "@/features/conversation/room/surface/mobile/room-mobile-overlay-frame";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";

export function TeamExecutionThread({job, name, avatar, compact, onClose, onOpenWorkspaceFile}: {
  job: TeamNodeJob; name: string; avatar?: string; compact: boolean; onClose: () => void; onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
}) {
  const { t } = useI18n();
  const [loadFailed, setLoadFailed] = useState(false);
  const [loading, setLoading] = useState(false);
  const sessionKey = buildRoomSharedSessionKey(job.conversation_id!);
  const identity = useMemo(() => ({session_key: sessionKey, room_id: job.room_id, conversation_id: job.conversation_id, chat_type: "group" as const}), [sessionKey, job.room_id, job.conversation_id]);
  const conversation = useAgentConversation({identity});
  const load = conversation.load_round_window;
  const reload = useCallback(async () => {
    setLoading(true);
    try { setLoadFailed(!await load(job.round_id!)); }
    catch { setLoadFailed(true); }
    finally { setLoading(false); }
  }, [load, job.round_id]);
  useEffect(() => { void reload(); }, [reload]);
  const messages = getRoomThreadMessages(conversation.messages.filter((item) => item.round_id === job.round_id), job.local_agent_id!);
  const permissions = conversation.pending_permissions.filter((item) => item.round_id === job.round_id && item.agent_id === job.local_agent_id);
  const panel = <ConversationThreadPanel agentId={job.local_agent_id!} agentName={name} agentAvatar={avatar}
    roundId={job.round_id!} messages={messages} sessionKey={sessionKey} workspaceAgentId={job.local_agent_id}
    pendingPermissions={permissions}
    onOpenWorkspaceFile={onOpenWorkspaceFile}
    footer={<>
      {loadFailed ? <div role="alert" className="p-3"><UiButton disabled={loading} onClick={() => { void reload(); }} variant="surface">{t("state.retry")}</UiButton></div> : null}
      {permissions.length ? <ComposerInteractionSurface permissions={permissions} onResponse={conversation.send_permission_response} fallbackAgentId={job.local_agent_id} agentNameMap={{[job.local_agent_id!]: name}} /> : null}
    </>}
    onStopMessage={(id) => {
      const message = messages.find((item) => item.message_id === id);
      if (message?.agent_round_id) conversation.stop_generation(message.agent_round_id);
    }}
    onPermissionResponse={conversation.send_permission_response} onClose={onClose}
    layout={compact ? "mobile" : "desktop"} presentation="inspector"
    isLoading={loading || conversation.is_session_loading || conversation.is_history_loading}
    emptyContent={<RoomThreadEmptyState isLoading={job.state === "running" || job.state === "ready"} />} />;
  return compact ? <RoomMobileOverlayFrame label={`${name} Thread`} onClose={onClose}>{panel}</RoomMobileOverlayFrame> : panel;
}
