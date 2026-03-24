import { useCallback, useEffect, useRef, useState } from 'react';
import { getAgentWsUrl } from '@/config/runtime-config';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { Message } from '@/types';
import { PermissionDecisionPayload } from '@/types/permission';
import {
  AgentConversationActionContext,
  AgentConversationLifecycleContext,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from '@/types/agent-conversation';
import {
  clearAgentConversation,
  loadAgentConversation,
  resetAgentConversation,
  startAgentConversation,
} from './session-lifecycle';
import { handleAgentConversationWebSocketMessage } from './websocket-event-handler';
import {
  deleteConversationRound,
  regenerateConversationRound,
  sendConversationMessage,
  sendConversationPermissionResponse,
  stopConversationGeneration,
} from './session-actions';

export function useAgentConversation(options: UseAgentConversationOptions = {}): UseAgentConversationReturn {
  const ws_url = options.ws_url || getAgentWsUrl();
  const apply_workspace_event = useWorkspaceLiveStore((state) => state.apply_event);

  const [messages, set_messages] = useState<Message[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const [session_key, set_session_key] = useState<string | null>(null);
  const [pending_permission, set_pending_permission] = useState<UseAgentConversationReturn['pending_permission']>(null);

  const active_session_key_ref = useRef<string | null>(null);
  const load_request_id_ref = useRef(0);
  const lifecycle_context: AgentConversationLifecycleContext = {
    active_session_key_ref,
    load_request_id_ref,
    set_session_key,
    set_messages,
    set_pending_permission,
    set_is_loading,
    set_error,
  };

  const is_current_session_event = useCallback((incoming_session_key?: string | null) => {
    if (!incoming_session_key) {
      return false;
    }
    return active_session_key_ref.current === incoming_session_key;
  }, []);

  const handle_websocket_message = useCallback((backend_message: unknown) => {
    handleAgentConversationWebSocketMessage({
      backend_message,
      apply_workspace_event,
      is_current_session_event,
      set_error,
      set_is_loading,
      set_messages,
      set_pending_permission,
    });
  }, [apply_workspace_event, is_current_session_event]);

  const has_connected_ref = useRef(false);

  const { state: ws_state, send: ws_send } = useWebSocket({
    url: ws_url,
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_websocket_message,
    on_error: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!has_connected_ref.current) {
        console.debug('[useAgentConversation] Ignored transient WebSocket error before first successful connection', event);
        return;
      }

      const error_message = 'WebSocket error occurred';
      console.error('[useAgentConversation] WebSocket error:', event);
      set_error(error_message);
      options.on_error?.(new Error(error_message));
    },
  });

  useEffect(() => {
    if (ws_state === 'connected') {
      has_connected_ref.current = true;
      set_error(null);
    }
  }, [ws_state]);

  useEffect(() => {
    const agent_id = options.agent_id;
    if (!agent_id || ws_state !== 'connected') {
      return;
    }

    ws_send({
      type: 'subscribe_workspace',
      agent_id,
    });

    return () => {
      ws_send({
        type: 'unsubscribe_workspace',
        agent_id,
      });
    };
  }, [options.agent_id, ws_send, ws_state]);

  const action_context: AgentConversationActionContext = {
    agent_id: options.agent_id,
    session_key,
    ws_state,
    ws_send,
    active_session_key_ref,
    pending_permission,
    messages,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_permission,
  };

  const send_message = useCallback(async (content: string) => {
    await sendConversationMessage(content, action_context);
  }, [action_context]);

  const stop_generation = useCallback(() => {
    stopConversationGeneration(action_context);
  }, [action_context]);

  const send_permission_response = useCallback((payload: PermissionDecisionPayload) => {
    sendConversationPermissionResponse(payload, action_context);
  }, [action_context]);

  const regenerate = useCallback(async (round_id: string) => {
    await regenerateConversationRound(round_id, action_context);
  }, [action_context]);

  const delete_round = useCallback(async (round_id: string) => {
    await deleteConversationRound(round_id, action_context);
  }, [action_context]);

  const start_session = useCallback(() => {
    startAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  const load_session = useCallback(async (id: string): Promise<void> => {
    await loadAgentConversation(id, lifecycle_context);
  }, [lifecycle_context]);

  const clear_session = useCallback(() => {
    clearAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  const reset_session = useCallback(() => {
    resetAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  return {
    error,
    messages,
    session_key,
    is_loading,
    pending_permission,
    send_message,
    start_session,
    load_session,
    clear_session,
    reset_session,
    stop_generation,
    delete_round,
    regenerate,
    send_permission_response,
  };
}

export type { UseAgentConversationOptions, UseAgentConversationReturn } from '@/types/agent-conversation';
