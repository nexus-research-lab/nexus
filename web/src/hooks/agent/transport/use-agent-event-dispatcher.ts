import { useCallback, useRef } from "react";

import type { AgentEventContext } from "./agent-event-context";
import { routeAgentConversationEvent } from "./agent-event-router";

/** Socket 订阅保持稳定，事件始终读取当前会话的最新上下文。 */
export function useAgentEventDispatcher(
  context: AgentEventContext,
): (backendMessage: unknown) => void {
  const contextRef = useRef(context);
  contextRef.current = context;

  return useCallback((backendMessage: unknown): void => {
    routeAgentConversationEvent(backendMessage, contextRef.current);
  }, []);
}
