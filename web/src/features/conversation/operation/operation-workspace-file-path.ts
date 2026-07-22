/**
 * INPUT: A file target emitted by either SDK plus the Agent workspace identity.
 * OUTPUT: A safe workspace-relative path suitable for Nexus file APIs.
 * POS: Operation Stage file trust boundary; absolute host paths never reach browser APIs.
 */

export interface ResolveOperationWorkspaceFilePathInput {
  agentId?: string | null;
  knownPaths?: readonly string[];
  path: string;
  workspacePath?: string | null;
}

export function resolveOperationWorkspaceFilePath({
  agentId,
  knownPaths = [],
  path,
  workspacePath,
}: ResolveOperationWorkspaceFilePathInput): string | null {
  const target = normalize_file_path(path);
  if (!target) {
    return null;
  }
  if (!is_absolute_path(target)) {
    return is_safe_relative_path(target) ? target : null;
  }

  const workspace_root = normalize_file_path(workspacePath ?? "");
  const relative_to_workspace = workspace_root
    ? strip_path_prefix(target, workspace_root)
    : null;
  if (relative_to_workspace && is_safe_relative_path(relative_to_workspace)) {
    return relative_to_workspace;
  }

  const nexus_workspace_path = relative_path_from_nexus_workspace(target, agentId);
  if (nexus_workspace_path) {
    return nexus_workspace_path;
  }

  const suffix_matches = knownPaths
    .map(normalize_file_path)
    .filter((candidate): candidate is string => Boolean(
      candidate
      && is_safe_relative_path(candidate)
      && path_ends_with(target, candidate),
    ));
  const unique_matches = Array.from(new Set(suffix_matches));
  return unique_matches.length === 1 ? unique_matches[0] : null;
}

function normalize_file_path(value: string): string | null {
  let normalized = value.trim().replace(/^file:\/\//i, "");
  try {
    normalized = decodeURIComponent(normalized);
  } catch {
    // Keep the original value when an SDK emits a literal percent character.
  }
  normalized = normalized
    .split(/[?#]/, 1)[0]
    ?.replace(/\\/g, "/")
    .replace(/^\.\//, "")
    .replace(/\/{2,}/g, "/")
    .replace(/\/$/, "") ?? "";
  return normalized || null;
}

function is_absolute_path(path: string): boolean {
  return path.startsWith("/") || /^[a-z]:\//i.test(path);
}

function is_safe_relative_path(path: string): boolean {
  if (!path || is_absolute_path(path)) {
    return false;
  }
  const segments = path.split("/");
  return path !== "." && !segments.some((segment) => segment === ".." || segment === "");
}

function strip_path_prefix(path: string, prefix: string): string | null {
  const is_windows_path = /^[a-z]:\//i.test(path) || /^[a-z]:\//i.test(prefix);
  const comparable_path = is_windows_path ? path.toLowerCase() : path;
  const comparable_prefix = is_windows_path ? prefix.toLowerCase() : prefix;
  if (comparable_path === comparable_prefix) {
    return null;
  }
  const prefix_with_separator = `${comparable_prefix}/`;
  return comparable_path.startsWith(prefix_with_separator)
    ? path.slice(prefix.length + 1)
    : null;
}

function relative_path_from_nexus_workspace(
  path: string,
  agent_id?: string | null,
): string | null {
  const segments = path.split("/").filter(Boolean);
  for (let index = 0; index < segments.length - 2; index += 1) {
    if (segments[index] === ".nexus" && segments[index + 1] === "workspace") {
      return relative_path_after_workspace_owner(segments, index + 2);
    }
    if (
      segments[index].startsWith(".nexus-")
      && segments[index + 1] === "instances"
      && segments[index + 3] === "workspace"
    ) {
      const workspace_owner_index = index + 4;
      if (segments[workspace_owner_index] !== agent_id?.trim()) {
        return null;
      }
      return relative_path_after_workspace_owner(segments, workspace_owner_index);
    }
  }
  return null;
}

function relative_path_after_workspace_owner(
  segments: string[],
  workspace_owner_index: number,
): string | null {
  const relative = segments.slice(workspace_owner_index + 1).join("/");
  return is_safe_relative_path(relative) ? relative : null;
}

function path_ends_with(path: string, candidate: string): boolean {
  const is_windows_path = /^[a-z]:\//i.test(path);
  const comparable_path = is_windows_path ? path.toLowerCase() : path;
  const comparable_candidate = is_windows_path ? candidate.toLowerCase() : candidate;
  return comparable_path.endsWith(`/${comparable_candidate}`);
}
