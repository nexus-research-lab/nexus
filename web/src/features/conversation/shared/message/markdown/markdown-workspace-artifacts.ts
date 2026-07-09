"use client";

import { useCallback, useMemo } from "react";

import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { type WorkspaceFileEntry } from "@/types/agent/agent";

export type ResolveWorkspaceFilePath = (value: string) => string | null;

interface ResolveWorkspaceArtifactPathOptions {
  allowUnlistedRelativePath?: boolean;
}

interface MarkdownTextSegment {
  type: "text";
  text: string;
}

interface MarkdownFileArtifactSegment {
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

function normalizeWorkspaceReference(value: string): string {
  return value
    .replace(/%60/gi, "`")
    .replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, "");
}

function looksLikeWorkspaceFileReference(value: string): boolean {
  if (!value.includes(".") || /^https?:\/\//i.test(value) || value.startsWith("/")) {
    return false;
  }

  return /[A-Za-z0-9]/.test(value);
}

function resolveWorkspaceFileReference(value: string, files: WorkspaceFileEntry[]): string | null {
  const normalized = normalizeWorkspaceReference(value);
  if (!looksLikeWorkspaceFileReference(normalized)) {
    return null;
  }

  const candidateFiles = files.filter((entry) => !entry.is_dir);
  const exactMatch = candidateFiles.find((entry) => entry.path === normalized);
  if (exactMatch) {
    return exactMatch.path;
  }

  const basenameMatches = candidateFiles.filter((entry) => entry.name === normalized);
  return basenameMatches.length === 1 ? basenameMatches[0].path : null;
}

function displayWorkspaceArtifactPath(path: string): string {
  const normalized = normalizeWorkspaceReference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.agent || !match.groups.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return `${match.groups.agent}/${match.groups.relative}`;
}

function clickableWorkspaceArtifactPath(path: string): string {
  const normalized = normalizeWorkspaceReference(path).replace(/\\/g, "/");
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalized);
  if (!match?.groups?.relative) {
    return normalized.replace(/^\.\//, "");
  }
  return match.groups.relative;
}

export function resolveWorkspaceArtifactPath(
  path: string,
  resolveFilePath: ResolveWorkspaceFilePath,
  options: ResolveWorkspaceArtifactPathOptions = {},
): string | null {
  const normalized = normalizeWorkspaceReference(path).replace(/\\/g, "/");
  if (WORKSPACE_ABSOLUTE_FILE_PATTERN.test(normalized)) {
    return clickableWorkspaceArtifactPath(normalized);
  }
  const resolvedPath = resolveFilePath(normalized);
  if (resolvedPath) {
    return resolvedPath;
  }
  if (
    options.allowUnlistedRelativePath &&
    (isWorkspaceRelativeArtifactPath(normalized) ||
      (isWorkspaceImagePath(normalized) && looksLikeWorkspaceFileReference(normalized)))
  ) {
    return normalized.replace(/^\.\//, "");
  }
  return null;
}

function normalizeArtifactLabel(prefix: string): string {
  const label = prefix.trim().replace(/[：:，,]$/, "").trim();
  return label || "已保存到";
}

function isWorkspaceImagePath(path: string): boolean {
  return WORKSPACE_IMAGE_EXTENSION_PATTERN.test(path.trim());
}

function isWorkspaceRelativeArtifactPath(path: string): boolean {
  const normalized = path.trim();
  return (
    looksLikeWorkspaceFileReference(normalized) &&
    normalized.includes("/") &&
    WORKSPACE_ARTIFACT_EXTENSION_PATTERN.test(normalized)
  );
}

export function splitMarkdownFileArtifacts(
  content: string,
  resolveFilePath: ResolveWorkspaceFilePath,
): MarkdownContentSegment[] {
  const segments: MarkdownContentSegment[] = [];
  const pendingText: string[] = [];

  const flushText = () => {
    if (pendingText.length === 0) {
      return;
    }
    segments.push({ type: "text", text: pendingText.join("\n") });
    pendingText.length = 0;
  };

  for (const line of content.split("\n")) {
    const match = SAVED_FILE_LINE_PATTERN.exec(line.trim());
    const absoluteMatch = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(line.trim());
    const path = match?.groups?.path ?? absoluteMatch?.groups?.path;
    if (!path) {
      pendingText.push(line);
      continue;
    }

    const resolvedPath = resolveWorkspaceArtifactPath(path, resolveFilePath, {
      allowUnlistedRelativePath: true,
    });
    if (!resolvedPath) {
      pendingText.push(line);
      continue;
    }

    flushText();
    segments.push({
      type: "file_artifact",
      label: match?.groups?.prefix ? normalizeArtifactLabel(match.groups.prefix) : "文件",
      path: resolvedPath,
      display_path: displayWorkspaceArtifactPath(path),
    });

    const suffix = match?.groups?.suffix?.trim() ?? "";
    if (suffix && /[\p{L}\p{N}]/u.test(suffix)) {
      pendingText.push(suffix);
    }
  }

  flushText();
  return segments.length > 0 ? segments : [{ type: "text", text: content }];
}

export function useMarkdownFileResolver(workspaceAgentId?: string | null): ResolveWorkspaceFilePath {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const filesByAgent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const resolvedAgentId = workspaceAgentId?.trim() || currentAgentId || "";
  const agentFiles = useMemo(
    () => filesByAgent[resolvedAgentId] ?? [],
    [filesByAgent, resolvedAgentId],
  );

  return useCallback(
    (value: string) => resolveWorkspaceFileReference(value, agentFiles),
    [agentFiles],
  );
}

export function useMarkdownCurrentAgentID(workspaceAgentId?: string | null): string | null {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  return workspaceAgentId?.trim() || currentAgentId;
}
