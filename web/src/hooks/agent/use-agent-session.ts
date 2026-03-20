import { useCallback, useEffect, useRef, useState } from 'react';
import { getAgentWsUrl } from '@/config/runtime-config';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { Message } from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';
import { UseAgentSessionOptions, UseAgentSessionReturn } from '@/types/agent-session';
import {
  AgentSessionActionContext,
  AgentSessionLifecycleContext,
} from './session-context';
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
  const wsUrl = options.wsUrl || getAgentWsUrl();
  const applyWorkspaceEvent = useWorkspaceLiveStore((state) => state.applyEvent);

  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sessionKey, setSessionKey] = useState<string | null>(null);
  const [pendingPermission, setPendingPermission] = useState<PendingPermission | null>(null);

  const activeSessionKeyRef = useRef<string | null>(null);
  const loadRequestIdRef = useRef(0);
  const lifecycleContext: AgentSessionLifecycleContext = {
    activeSessionKeyRef,
    loadRequestIdRef,
    setSessionKey,
    setMessages,
    setPendingPermission,
    setIsLoading,
    setError,
  };

  const isCurrentSessionEvent = useCallback((incomingSessionKey?: string | null) => {
    if (!incomingSessionKey) {
      return false;
    }
    return activeSessionKeyRef.current === incomingSessionKey;
  }, []);

  const handleWebSocketMessage = useCallback((backendMessage: unknown) => {
    handleAgentWebSocketMessage({
      backendMessage,
      applyWorkspaceEvent,
      isCurrentSessionEvent,
      setError,
      setIsLoading,
      setMessages,
      setPendingPermission,
    });
  }, [applyWorkspaceEvent, isCurrentSessionEvent]);

  const hasConnectedRef = useRef(false);

  const { state: wsState, send: wsSend } = useWebSocket({
    url: wsUrl,
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
    onMessage: handleWebSocketMessage,
    onError: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!hasConnectedRef.current) {
        console.debug('[useAgentSession] Ignored transient WebSocket error before first successful connection', event);
        return;
      }

      const errorMessage = 'WebSocket error occurred';
      console.error('[useAgentSession] WebSocket error:', event);
      setError(errorMessage);
      options.onError?.(new Error(errorMessage));
    },
  });

  useEffect(() => {
    if (wsState === 'connected') {
      hasConnectedRef.current = true;
      setError(null);
    }
  }, [wsState]);

  useEffect(() => {
    const agentId = options.agentId;
    if (!agentId || wsState !== 'connected') {
      return;
    }

    wsSend({
      type: 'subscribe_workspace',
      agent_id: agentId,
    });

    return () => {
      wsSend({
        type: 'unsubscribe_workspace',
        agent_id: agentId,
      });
    };
  }, [options.agentId, wsSend, wsState]);

  const actionContext: AgentSessionActionContext = {
    agentId: options.agentId,
    sessionKey,
    wsState,
    wsSend,
    activeSessionKeyRef,
    pendingPermission,
    messages,
    setError,
    setIsLoading,
    setMessages,
    setPendingPermission,
  };

  const sendMessage = useCallback(async (content: string) => {
    await sendSessionMessage(content, actionContext);
  }, [actionContext]);

  const stopGeneration = useCallback(() => {
    stopSessionGeneration(actionContext);
  }, [actionContext]);

  const sendPermissionResponse = useCallback((payload: PermissionDecisionPayload) => {
    sendSessionPermissionResponse(payload, actionContext);
  }, [actionContext]);

  const regenerate = useCallback(async (roundId: string) => {
    await regenerateSessionRound(roundId, actionContext);
  }, [actionContext]);

  const deleteRound = useCallback(async (roundId: string) => {
    await deleteSessionRound(roundId, actionContext);
  }, [actionContext]);

  const startSession = useCallback(() => {
    startAgentSession(lifecycleContext);
  }, [lifecycleContext]);

  const loadSession = useCallback(async (id: string): Promise<void> => {
    await loadAgentSession(id, lifecycleContext);
  }, [lifecycleContext]);

  const clearSession = useCallback(() => {
    clearAgentSession(lifecycleContext);
  }, [lifecycleContext]);

  const resetSession = useCallback(() => {
    resetAgentSession(lifecycleContext);
  }, [lifecycleContext]);

  return {
    error,
    messages,
    sessionKey,
    isLoading,
    pendingPermission,
    sendMessage,
    startSession,
    loadSession,
    clearSession,
    resetSession,
    stopGeneration,
    deleteRound,
    regenerate,
    sendPermissionResponse,
  };
}

export type { UseAgentSessionOptions, UseAgentSessionReturn } from '@/types/agent-session';
