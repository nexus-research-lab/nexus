/**
 * INPUT: Provider 原生 tool_result、宿主缓存的紧凑 metadata 与历史 JSON 文本。
 * OUTPUT: 与 Go protocol 一致的 applied/no_op/rejected/superseded mutation 语义。
 * POS: DM、Room、Tool 卡片与过程摘要共用的只读投影；不决定 Agent 是否或如何重试。
 */
import type { ToolResultContent } from "@/types/conversation/message/content";

const MUTATION_RESULT_JSON_LIMIT = 64 * 1024;
const MUTATION_OUTCOMES = new Set<MutationResultOutcome>([
  "applied",
  "no_op",
  "rejected",
  "superseded",
]);

const MUTATION_OUTCOME_METADATA_KEY = "_nexus_mutation_outcome";
const MUTATION_MESSAGE_METADATA_KEY = "_nexus_mutation_message";
const MUTATION_REASON_CODE_METADATA_KEY = "_nexus_mutation_reason_code";

export type MutationResultOutcome =
  | "applied"
  | "no_op"
  | "rejected"
  | "superseded";

export interface MutationResultSemantic {
  message: string;
  outcome: MutationResultOutcome;
  reasonCode: string;
}

export function projectToolResultMutation(
  result: ToolResultContent | undefined,
): MutationResultSemantic | null {
  if (!result) {
    return null;
  }
  const metadata = asRecord(result.metadata);
  return firstMutationResult([
    metadata
      ? {
          message: metadata[MUTATION_MESSAGE_METADATA_KEY],
          outcome: metadata[MUTATION_OUTCOME_METADATA_KEY],
          reason_code: metadata[MUTATION_REASON_CODE_METADATA_KEY],
        }
      : null,
    result.structured_output,
    result.content,
  ]);
}

export function isRejectedToolResult(
  result: ToolResultContent | undefined,
): boolean {
  return projectToolResultMutation(result)?.outcome === "rejected";
}

export function isSupersededToolResult(
  result: ToolResultContent | undefined,
): boolean {
  return projectToolResultMutation(result)?.outcome === "superseded";
}

function firstMutationResult(
  values: readonly unknown[],
): MutationResultSemantic | null {
  for (const value of values) {
    const result = parseMutationResult(value, 0);
    if (result) {
      return result;
    }
  }
  return null;
}

function parseMutationResult(
  value: unknown,
  depth: number,
): MutationResultSemantic | null {
  if (value == null || depth > 3) {
    return null;
  }
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed || trimmed.length > MUTATION_RESULT_JSON_LIMIT) {
      return null;
    }
    try {
      return parseMutationResult(JSON.parse(trimmed), depth + 1);
    } catch {
      return null;
    }
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const result = parseMutationResult(item, depth + 1);
      if (result) {
        return result;
      }
    }
    return null;
  }
  const record = asRecord(value);
  if (!record) {
    return null;
  }
  const outcome = stringValue(record.outcome) as MutationResultOutcome;
  if (MUTATION_OUTCOMES.has(outcome)) {
    return {
      message: stringValue(record.message),
      outcome,
      reasonCode: stringValue(record.reason_code),
    };
  }
  for (const key of [
    "structured_output",
    "structured_content",
    "structuredContent",
    "content",
    "text",
  ]) {
    const result = parseMutationResult(record[key], depth + 1);
    if (result) {
      return result;
    }
  }
  return null;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
