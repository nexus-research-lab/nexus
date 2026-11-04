// INPUT: Explicit message fields and localized React elements for conversation tests.
// OUTPUT: Small message fixtures and server-rendered markup; no transport or state simulation.
// POS: Test-only data shared by conversation regression suites.
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

export function userMessage({
  agentRoundId,
  content,
  deliveryPolicy,
  messageId,
  roundId,
  sourceRoundId,
  targetAgentIds,
  timestamp,
}) {
  return {
    agent_id: "",
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content,
    ...(deliveryPolicy ? { delivery_policy: deliveryPolicy } : {}),
    message_id: messageId,
    role: "user",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    ...(sourceRoundId ? { source_round_id: sourceRoundId } : {}),
    ...(targetAgentIds ? { target_agent_ids: targetAgentIds } : {}),
    timestamp,
  };
}

export function assistantMessage({
  agentId = "agent-1",
  agentRoundId,
  displayOrder,
  isComplete = false,
  messageId = "assistant-root",
  model,
  resultSummary,
  roundId = "round-root",
  status = "streaming",
  stopReason,
  text,
  timestamp,
}) {
  return {
    agent_id: agentId,
    ...(agentRoundId ? { agent_round_id: agentRoundId } : {}),
    content: [{ type: "text", text }],
    ...(displayOrder === undefined ? {} : { display_order: displayOrder }),
    is_complete: isComplete,
    message_id: messageId,
    ...(model ? { model } : {}),
    ...(resultSummary ? { result_summary: resultSummary } : {}),
    role: "assistant",
    round_id: roundId,
    session_key: "room:group:conversation-1",
    ...(stopReason ? { stop_reason: stopReason } : {}),
    stream_status: status,
    timestamp,
  };
}

export async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await import("@/shared/i18n/i18n-context.ts");
  return renderToStaticMarkup(React.createElement(
    I18N_CONTEXT.Provider,
    { value: await loadI18nValue(locale) },
    element,
  ));
}

export async function loadI18nValue(locale = "zh") {
  const { MESSAGES } = await import("@/shared/i18n/messages.ts");
  return {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, value]) => message.replaceAll(
        `{${name}}`,
        String(value),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
}
