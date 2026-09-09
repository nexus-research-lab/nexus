// INPUT: Provider tool invocation failure text.
// OUTPUT: Shared bounded failure notice preserving diagnostic issues.
// POS: Tool invocation error projection; no recovery or execution authority.
"use client";

import { AlertTriangle } from "lucide-react";

import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { useI18n } from "@/shared/i18n/i18n-context";

const TOOL_USE_ERROR_PATTERN = /^(?:(?<error_type>[A-Za-z]+Error):\s*)?(?<tool_name>[A-Za-z][A-Za-z0-9_]*) failed due to the following issues:\s*(?<issues>[\s\S]*)$/;

interface ParsedToolUseError {
  error_type: string;
  tool_name: string;
  issues: string[];
}

interface ToolUseErrorBlockProps {
  content: string;
}

function parseToolUseError(
  content: string,
  fallbackIssue: string,
  incompleteIssue: string,
): ParsedToolUseError {
  const normalized = content.trim();
  const match = TOOL_USE_ERROR_PATTERN.exec(normalized);
  if (!match?.groups) {
    return {
      error_type: "ToolUseError",
      tool_name: "Tool",
      issues: normalized ? [normalized] : [fallbackIssue],
    };
  }

  const issues = match.groups.issues
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean);

  return {
    error_type: match.groups.error_type || "ToolUseError",
    tool_name: match.groups.tool_name,
    issues: issues.length > 0 ? issues : [incompleteIssue],
  };
}

export function ToolUseErrorBlock({ content }: ToolUseErrorBlockProps) {
  const { t } = useI18n();
  const parsed = parseToolUseError(
    content,
    t("message.tool_call_failed"),
    t("message.tool_parameters_incomplete"),
  );

  return (
    <UiInlineNotice
      className="my-2"
      icon={<AlertTriangle aria-hidden />}
      title={t("message.tool_invocation_failed", { tool: parsed.tool_name })}
      message={(
        <>
          <code className="block break-all">{parsed.error_type}</code>
          {parsed.issues.map((issue, index) => (
            <span key={`${index}-${issue}`} className="block break-words">{issue}</span>
          ))}
        </>
      )}
      tone="danger"
      width="compact"
    />
  );
}
