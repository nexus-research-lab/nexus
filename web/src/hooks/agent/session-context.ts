import { Dispatch, RefObject, SetStateAction } from 'react';

import { WebSocketMessage, WebSocketState } from '@/lib/websocket';
import { Message } from '@/types';
import { PendingPermission } from '@/types/permission';

export interface AgentSessionActionContext {
  agentId?: string | null;
  sessionKey: string | null;
  wsState: WebSocketState;
  wsSend: (message: WebSocketMessage) => void;
  activeSessionKeyRef: RefObject<string | null>;
  pendingPermission: PendingPermission | null;
  messages: Message[];
  setError: Dispatch<SetStateAction<string | null>>;
  setIsLoading: Dispatch<SetStateAction<boolean>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermission: Dispatch<SetStateAction<PendingPermission | null>>;
}

export interface AgentSessionLifecycleContext {
  activeSessionKeyRef: RefObject<string | null>;
  loadRequestIdRef: RefObject<number>;
  setSessionKey: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingPermission: Dispatch<SetStateAction<PendingPermission | null>>;
  setIsLoading: Dispatch<SetStateAction<boolean>>;
  setError: Dispatch<SetStateAction<string | null>>;
}
