"use client";

import { AlertTriangle } from "lucide-react";

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
    <div className="my-2 min-w-0 border-l-2 border-(--destructive) pl-4">
      <div className="message-cjk-font flex min-w-0 items-start gap-2 py-1 text-xs">
        <span
          data-timeline-anchor
          data-timeline-anchor-mode="box"
          className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-(--destructive)"
        >
          <AlertTriangle className="h-3.5 w-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-xs font-medium text-(--destructive)">
              {t("message.tool_invocation_failed", {
                tool: parsed.tool_name,
              })}
            </span>
            <span className="shrink-0 text-xs text-(--text-soft)">
              {parsed.error_type}
            </span>
          </div>
          <div className="mt-1 space-y-0.5 text-compact leading-5 text-(--text-muted)">
            {parsed.issues.map((issue, index) => (
              <div key={`${index}-${issue}`} className="break-words">
                {issue}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
