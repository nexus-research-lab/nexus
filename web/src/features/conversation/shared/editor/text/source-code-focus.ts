/**
 * INPUT: Workspace source text and an optional tool-owned focus range or changed snippets.
 * OUTPUT: Merged, clamped source line ranges for truthful operation highlighting.
 * POS: Pure source focus model shared by the text editor view and operation semantics tests.
 */
import type { WorkspaceSourceFocus } from "../workspace-file-preview-types";

export interface SourceLineRange {
  end: number;
  start: number;
}

export function resolveSourceFocusRanges(
  content: string,
  focus?: WorkspaceSourceFocus | null,
): SourceLineRange[] {
  if (!focus) {
    return [];
  }
  const line_count = Math.max(1, content.split("\n").length);
  const ranges: SourceLineRange[] = [];
  if (is_positive_integer(focus.startLine)) {
    const start = clamp_line(focus.startLine, line_count);
    const end = is_positive_integer(focus.endLine)
      ? clamp_line(Math.max(start, focus.endLine), line_count)
      : start;
    ranges.push({ start, end });
  }
  for (const snippet of focus.snippets ?? []) {
    if (!snippet) {
      continue;
    }
    const index = content.indexOf(snippet);
    if (index < 0) {
      continue;
    }
    const start = content.slice(0, index).split("\n").length;
    const end = Math.min(line_count, start + snippet.split("\n").length - 1);
    ranges.push({ start, end });
  }
  return merge_ranges(ranges);
}

function clamp_line(value: number, line_count: number): number {
  return Math.min(line_count, Math.max(1, Math.floor(value)));
}

function is_positive_integer(value: number | null | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 1;
}

function merge_ranges(ranges: SourceLineRange[]): SourceLineRange[] {
  const ordered = ranges
    .slice()
    .sort((left, right) => left.start - right.start || left.end - right.end);
  const merged: SourceLineRange[] = [];
  for (const range of ordered) {
    const previous = merged.at(-1);
    if (!previous || range.start > previous.end + 1) {
      merged.push({ ...range });
      continue;
    }
    previous.end = Math.max(previous.end, range.end);
  }
  return merged;
}
