import type {
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";

import type { NexusOperationEvent } from "./operation-types";
import {
  OPERATION_MAX_TEXT_PREVIEW,
  redactProjectedValue,
} from "./operation-projection-preview";

export function projectResultSummaryEvent({
  message,
  projected_messages,
}: {
  message: Extract<Message, { role: "assistant" }>;
  projected_messages: Message[];
}): NexusOperationEvent | null {
  if (!message.result_summary) {
    return null;
  }

  const is_summary_error = is_result_summary_error(message);
  const summary_text = message.result_summary.result ?? extract_assistant_text_preview(message) ?? null;
  const result_preview = build_summary_result_preview(
    message.result_summary,
    is_summary_error,
    summary_text,
  );
  const round_started_at = find_round_start_timestamp(
    projected_messages,
    message.round_id,
    message.timestamp,
  );

  return {
    id: `${message.message_id}:summary`,
    session_key: message.session_key,
    round_id: message.round_id,
    agent_id: message.agent_id,
    message_id: message.message_id,
    kind: "round_summary",
    surface: "summary",
    phase: is_summary_error
      ? "error"
      : message.result_summary.subtype === "interrupted"
        ? "cancelled"
        : "done",
    title: is_summary_error ? "本轮执行异常" : "本轮执行收口",
    target: `${message.result_summary.num_turns} turns`,
    summary: summary_text,
    result_preview,
    evidence: [
      ...(is_summary_error ? [{ type: "error" as const, label: "error", value: summary_text }] : []),
      { type: "status", label: "duration", value: `${Math.round(message.result_summary.duration_ms / 1000)}s` },
      { type: "status", label: "turns", value: String(message.result_summary.num_turns) },
    ],
    started_at: round_started_at,
    updated_at: message.result_summary.timestamp ?? message.timestamp,
    ended_at: message.result_summary.timestamp ?? message.timestamp,
  };
}

export function projectImplicitRoundSummaryEvents({
  messages,
  live_round_ids,
}: {
  messages: Message[];
  live_round_ids: ReadonlySet<string>;
}): NexusOperationEvent[] {
  const assistant_messages_by_round = new Map<string, Array<Extract<Message, { role: "assistant" }>>>();

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    const round_messages = assistant_messages_by_round.get(message.round_id) ?? [];
    round_messages.push(message);
    assistant_messages_by_round.set(message.round_id, round_messages);
  }

  const summaries: NexusOperationEvent[] = [];
  for (const [round_id, round_messages] of assistant_messages_by_round) {
    if (
      live_round_ids.has(round_id) ||
      round_messages.some((message) => message.result_summary) ||
      !round_messages.some(message_has_tool_use)
    ) {
      continue;
    }

    const terminal_message = [...round_messages].reverse().find(is_implicit_terminal_message);
    if (!terminal_message) {
      continue;
    }

    const phase = terminal_message.stream_status === "error"
      ? "error"
      : terminal_message.stream_status === "cancelled"
        ? "cancelled"
        : "done";
    const summary_text = extract_assistant_text_preview(terminal_message)
      ?? (phase === "done" ? "本轮执行已完成。" : "本轮执行未完成。");
    const started_at = find_round_start_timestamp(
      messages,
      round_id,
      terminal_message.timestamp,
    );
    const duration_ms = Math.max(0, terminal_message.timestamp - started_at);

    summaries.push({
      id: `${terminal_message.message_id}:implicit-summary`,
      session_key: terminal_message.session_key,
      round_id,
      agent_id: terminal_message.agent_id,
      message_id: terminal_message.message_id,
      kind: "round_summary",
      surface: "summary",
      phase,
      title: phase === "done" ? "本轮执行收口" : "本轮执行异常",
      target: `${round_messages.length} turns`,
      summary: summary_text,
      result_preview: summary_text,
      evidence: [
        ...(duration_ms > 0
          ? [{ type: "status" as const, label: "duration", value: `${Math.round(duration_ms / 1000)}s` }]
          : []),
        { type: "status", label: "turns", value: String(round_messages.length) },
      ],
      duration_ms,
      started_at,
      updated_at: terminal_message.timestamp,
      ended_at: terminal_message.timestamp,
    });
  }

  return summaries;
}

function build_summary_result_preview(
  summary: ResultSummary,
  is_error: boolean,
  summary_text: string | null,
): unknown {
  const redacted = redactProjectedValue(summary) as Record<string, unknown>;
  if (!is_error) {
    return redacted;
  }

  return {
    ...redacted,
    is_error: true,
    result: summary_text,
    subtype: summary.subtype === "interrupted" ? "interrupted" : "error",
  };
}

function is_result_summary_error(message: Extract<Message, { role: "assistant" }>): boolean {
  if (!message.result_summary) {
    return false;
  }
  if (message.result_summary.is_error || message.result_summary.subtype === "error") {
    return true;
  }
  if (message.stream_status === "error") {
    return true;
  }
  if (message.model === "<synthetic>") {
    const text = extract_assistant_text_preview(message) ?? "";
    return /\b(error|failed|unauthorized|authenticate|invalid|expired)\b/i.test(text);
  }
  return false;
}

function message_has_tool_use(message: Extract<Message, { role: "assistant" }>): boolean {
  return message.content.some((block) => block.type === "tool_use");
}

function is_implicit_terminal_message(message: Extract<Message, { role: "assistant" }>): boolean {
  if (message.stream_status === "error" || message.stream_status === "cancelled") {
    return true;
  }
  if (message.stop_reason === "tool_use") {
    return false;
  }
  return message.stream_status === "done"
    || message.is_complete === true;
}

function extract_assistant_text_preview(message: Extract<Message, { role: "assistant" }>): string | null {
  const text = message.content
    .filter((block) => block.type === "text")
    .map((block) => block.text)
    .join("\n")
    .trim();
  if (!text) {
    return null;
  }
  return text.length > OPERATION_MAX_TEXT_PREVIEW ? `${text.slice(0, OPERATION_MAX_TEXT_PREVIEW)}...` : text;
}

function find_round_start_timestamp(
  messages: Message[],
  round_id: string,
  fallback_timestamp: number,
): number {
  const first_round_message = messages.find((message) => message.round_id === round_id);
  return first_round_message?.timestamp ?? fallback_timestamp;
}
