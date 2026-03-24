import { useCallback, useEffect, useRef, useState } from 'react';
import { getAgentWsUrl } from '@/config/runtime-config';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { Message } from '@/types';
import { PermissionDecisionPayload } from '@/types/permission';
import {
  AgentSessionActionContext,
  AgentSessionLifecycleContext,
  UseAgentSessionOptions,
  UseAgentSessionReturn,
} from '@/types/agent-session';
import {
  clearAgentSession,
  loadAgentSession,
  resetAgentSession,
  startAgentSession,
} from './session-lifecycle';
import { handleAgentWebSocketMessage } from './websocket-event-handler';
import {
  deleteSessionRound,
  regenerateSessionRound,
  sendSessionMessage,
  sendSessionPermissionResponse,
  stopSessionGeneration,
} from './session-actions';

export function useAgentSession(options: UseAgentSessionOptions = {}): UseAgentSessionReturn {
  const ws_url = options.ws_url || getAgentWsUrl();
  const apply_workspace_event = useWorkspaceLiveStore((state) => state.applyEvent);

  const [messages, set_messages] = useState<Message[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const [session_key, set_session_key] = useState<string | null>(null);
  const [pending_permission, set_pending_permission] = useState<UseAgentSessionReturn['pending_permission']>(null);

  const active_session_key_ref = useRef<string | null>(null);
  const load_request_id_ref = useRef(0);
  const lifecycle_context: AgentSessionLifecycleContext = {
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
    handleAgentWebSocketMessage({
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
    autoConnect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    onMessage: handle_websocket_message,
    onError: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!has_connected_ref.current) {
        console.debug('[useAgentSession] Ignored transient WebSocket error before first successful connection', event);
        return;
      }

      const error_message = 'WebSocket error occurred';
      console.error('[useAgentSession] WebSocket error:', event);
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

  const action_context: AgentSessionActionContext = {
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
    await sendSessionMessage(content, action_context);
  }, [action_context]);

  const stop_generation = useCallback(() => {
    stopSessionGeneration(action_context);
  }, [action_context]);

  const send_permission_response = useCallback((payload: PermissionDecisionPayload) => {
    sendSessionPermissionResponse(payload, action_context);
  }, [action_context]);

  const regenerate = useCallback(async (round_id: string) => {
    await regenerateSessionRound(round_id, action_context);
  }, [action_context]);

  const delete_round = useCallback(async (round_id: string) => {
    await deleteSessionRound(round_id, action_context);
  }, [action_context]);

  const start_session = useCallback(() => {
    startAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  const load_session = useCallback(async (id: string): Promise<void> => {
    await loadAgentSession(id, lifecycle_context);
  }, [lifecycle_context]);

  const clear_session = useCallback(() => {
    clearAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  const reset_session = useCallback(() => {
    resetAgentSession(lifecycle_context);
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

export type { UseAgentSessionOptions, UseAgentSessionReturn } from '@/types/agent-session';
