import type { AgentEventHandler } from "../agent-event-context";

/** Session 事件必须明确属于当前会话，缺失或过期作用域直接忽略。 */
export function withCurrentSessionEvent(
  handler: AgentEventHandler,
): AgentEventHandler {
  return (event, context) => {
    if (!context.scope.isCurrentSessionEvent(event.session_key || null)) {
      return;
    }
    handler(event, context);
  };
}
