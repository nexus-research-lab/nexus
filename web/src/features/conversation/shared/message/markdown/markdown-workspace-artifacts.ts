"use client";

import { useCallback, useMemo } from "react";

import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { type WorkspaceFileEntry } from "@/types/agent/agent";

export type ResolveWorkspaceFilePath = (value: string) => string | null;

export interface MarkdownTextSegment {
  type: "text";
  text: string;
}

export interface MarkdownFileArtifactSegment {
  type: "file_artifact";
  label: string;
  path: string;
  display_path: string;
}

export type MarkdownContentSegment = MarkdownTextSegment | MarkdownFileArtifactSegment;

const WORKSPACE_ABSOLUTE_FILE_PATTERN = /(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/(?<agent>[^/\s`"'，。；！？]+)\/(?<relative>[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}))/;
const SAVED_FILE_LINE_PATTERN = /^(?<prefix>.*?(?:已保存到|保存到|写入到|生成到|created at|saved to|written to)\s*)[`"']?(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/[^/\s`"'，。；！？]+\/[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}|[A-Za-z0-9_.-][A-Za-z0-9_./-]*\.[A-Za-z0-9]{1,10})[`"']?(?<suffix>.*)$/i;
const WORKSPACE_ARTIFACT_EXTENSION_PATTERN = /\.(?:adoc|avif|bmp|csv|gif|html?|ico|jpe?g|jsonl?|log|markdown|md|mermaid|mmd|pdf|png|rst|svg|toml|txt|webp|xml|ya?ml)$/i;
const WORKSPACE_IMAGE_EXTENSION_PATTERN = /\.(?:png|jpe?g|webp|gif|avif)$/i;

function normalize_workspace_reference(value: string): string {
  return value
    .replace(/%60/gi, "`")
    .replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, "");
}

function looks_like_workspace_file_reference(value: string): boolean {
  if (!value.includes(".") || /^https?:\/\//i.test(value) || value.startsWith("/")) {
    return false;
  }

  return /[A-Za-z0-9]/.test(value);
}

function resolve_workspace_file_reference(value: string, files: WorkspaceFileEntry[]): string | null {
  const normalized = normalize_workspace_reference(value);
  if (!looks_like_workspace_file_reference(normalized)) {
    return null;
  }

  const candidate_files = files.filter((entry) => !entry.is_dir);
  const exact_match = candidate_files.find((entry) => entry.path === normalized);
  if (exact_match) {
    return exact_match.path;
  }

  const basename_matches = candidate_files.filter((entry) => entry.name === normalized);
  return basename_matches.length === 1 ? basename_matches[0].path : null;
}

function display_workspace_artifact_path(path: string): string {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.agent || !match.groups.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return `${match.groups.agent}/${match.groups.relative}`;
}

function clickable_workspace_artifact_path(path: string): string {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return match.groups.relative;
}

export function resolve_workspace_artifact_path(
  path: string,
  resolve_file_path: ResolveWorkspaceFilePath,
): string | null {
  const normalized = normalize_workspace_reference(path).replace(/\\/g, "/");
  if (WORKSPACE_ABSOLUTE_FILE_PATTERN.test(normalized)) {
    return clickable_workspace_artifact_path(normalized);
  }
  const resolved_path = resolve_file_path(normalized);
  if (resolved_path) {
    return resolved_path;
  }
  if (is_workspace_relative_artifact_path(normalized)) {
    return normalized.replace(/^\.\//, "");
  }
  if (is_workspace_image_path(normalized) && looks_like_workspace_file_reference(normalized)) {
    return normalized.replace(/^\.\//, "");
  }
  return null;
}

function normalize_artifact_label(prefix: string): string {
  const label = prefix.trim().replace(/[：:，,]$/, "").trim();
  return label || "已保存到";
}

function is_workspace_image_path(path: string): boolean {
  return WORKSPACE_IMAGE_EXTENSION_PATTERN.test(path.trim());
}

function is_workspace_relative_artifact_path(path: string): boolean {
  const normalized = path.trim();
  return (
    looks_like_workspace_file_reference(normalized) &&
    normalized.includes("/") &&
    WORKSPACE_ARTIFACT_EXTENSION_PATTERN.test(normalized)
  );
}

export function split_markdown_file_artifacts(
  content: string,
  resolve_file_path: ResolveWorkspaceFilePath,
): MarkdownContentSegment[] {
  const segments: MarkdownContentSegment[] = [];
  const pending_text: string[] = [];

  const flush_text = () => {
    if (pending_text.length === 0) {
      return;
    }
    segments.push({ type: "text", text: pending_text.join("\n") });
    pending_text.length = 0;
  };

  for (const line of content.split("\n")) {
    const match = SAVED_FILE_LINE_PATTERN.exec(line.trim());
    const absolute_match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(line.trim());
    const path = match?.groups?.path ?? absolute_match?.groups?.path;
    if (!path) {
      pending_text.push(line);
      continue;
    }

    const resolved_path = resolve_workspace_artifact_path(path, resolve_file_path);
    if (!resolved_path) {
      pending_text.push(line);
      continue;
    }

    flush_text();
    segments.push({
      type: "file_artifact",
      label: match?.groups?.prefix ? normalize_artifact_label(match.groups.prefix) : "文件",
      path: resolved_path,
      display_path: display_workspace_artifact_path(path),
    });

    const suffix = match?.groups?.suffix?.trim() ?? "";
    if (suffix && /[\p{L}\p{N}]/u.test(suffix)) {
      pending_text.push(suffix);
    }
  }

  flush_text();
  return segments.length > 0 ? segments : [{ type: "text", text: content }];
}

export function useMarkdownFileResolver(workspace_agent_id?: string | null): ResolveWorkspaceFilePath {
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const resolved_agent_id = workspace_agent_id?.trim() || current_agent_id || "";
  const agent_files = useMemo(
    () => files_by_agent[resolved_agent_id] ?? [],
    [files_by_agent, resolved_agent_id],
  );

  return useCallback(
    (value: string) => resolve_workspace_file_reference(value, agent_files),
    [agent_files],
  );
}

export function useMarkdownCurrentAgentID(workspace_agent_id?: string | null): string | null {
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  return workspace_agent_id?.trim() || current_agent_id;
}
