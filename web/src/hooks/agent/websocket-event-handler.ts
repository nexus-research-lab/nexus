import { Dispatch, SetStateAction } from 'react';

import { EventMessage, Message, StreamMessage } from '@/types';
import { PendingPermission } from '@/types/permission';
import { WorkspaceEventPayload } from '@/types/workspace-live';

import { applyStreamMessage, upsertMessage } from './message-helpers';

interface HandleAgentWebSocketMessageParams {
  backendMessage: unknown;
  applyWorkspaceEvent: (payload: WorkspaceEventPayload) => void;
  isCurrentSessionEvent: (incomingSessionKey?: string | null) => boolean;
  setError: Dispatch<SetStateAction<string | null>>;
  setIsLoading: Dispatch<SetStateAction<boolean>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermission: Dispatch<SetStateAction<PendingPermission | null>>;
}

/**
 * 处理 Agent 会话的 WebSocket 事件。
 */
export function handleAgentWebSocketMessage({
  backendMessage,
  applyWorkspaceEvent,
  isCurrentSessionEvent,
  setError,
  setIsLoading,
  setMessages,
  setPendingPermission,
}: HandleAgentWebSocketMessageParams): void {
  const event = backendMessage as EventMessage;
  const incomingSessionKey = event.session_key || null;

  if (event.event_type === 'error') {
    if (incomingSessionKey && !isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    setError(event.data?.message || 'Unknown error');
    setIsLoading(false);
    return;
  }

  if (event.event_type === 'permission_request') {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const data = event.data || {};
    setPendingPermission({
      request_id: data.request_id,
      tool_name: data.tool_name,
      tool_input: data.tool_input || {},
      risk_level: data.risk_level,
      risk_label: data.risk_label,
      summary: data.summary,
      suggestions: data.suggestions || [],
      expires_at: data.expires_at,
    });
    return;
  }

  if (event.event_type === 'workspace_event') {
    const payload = event.data as WorkspaceEventPayload;
    if (payload?.agent_id && payload?.path) {
      applyWorkspaceEvent(payload);
    }
    return;
  }

  if (event.event_type !== 'message') {
    if (event.event_type !== 'stream') {
      return;
    }

    const payload = event.data as StreamMessage;
    const messageSessionKey = payload?.session_key || incomingSessionKey;
    if (!payload || !messageSessionKey || !isCurrentSessionEvent(messageSessionKey)) {
      return;
    }

    setMessages((prev) => applyStreamMessage(prev, payload));
    setIsLoading(true);
    return;
  }

  const payload = event.data as Message;
  const messageSessionKey = payload?.session_key || incomingSessionKey;
  if (!payload || !messageSessionKey || !isCurrentSessionEvent(messageSessionKey)) {
    return;
  }

  setMessages((prev) => upsertMessage(prev, payload));
  if (payload.role === 'result') {
    setPendingPermission(null);
    setIsLoading(false);
    return;
  }
  if (payload.role === 'assistant') {
    setIsLoading(true);
  }
}
