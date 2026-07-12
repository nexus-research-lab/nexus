import type { WorkspaceFileEntry } from "@/types/agent/agent";

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

export type MarkdownContentSegment =
  | MarkdownFileArtifactSegment
  | MarkdownTextSegment;

interface ArtifactPathContext {
  allowUnlistedRelativePath: boolean;
  normalizedPath: string;
  resolveFilePath: ResolveWorkspaceFilePath;
}

interface ParsedArtifactLine {
  artifact: MarkdownFileArtifactSegment;
  leadingText: string | null;
  trailingText: string | null;
}

type ArtifactLineParser = (
  line: string,
  resolveFilePath: ResolveWorkspaceFilePath,
) => ParsedArtifactLine | null;

interface MarkdownSegmentAccumulator {
  pendingText: string[];
  segments: MarkdownContentSegment[];
}

const WORKSPACE_ABSOLUTE_FILE_PATTERN = /(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/(?<agent>[^/\s`"'，。；！？]+)\/(?<relative>[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}))/;
const SAVED_FILE_LINE_PATTERN = /^(?<prefix>.*?(?:已保存到|保存到|写入到|生成到|created at|saved to|written to)\s*)[`"']?(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/[^/\s`"'，。；！？]+\/[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}|[A-Za-z0-9_.-][A-Za-z0-9_./-]*\.[A-Za-z0-9]{1,10})[`"']?(?<suffix>.*)$/i;
const WORKSPACE_ARTIFACT_EXTENSION_PATTERN = /\.(?:adoc|avif|bmp|csv|gif|html?|ico|jpe?g|jsonl?|log|markdown|md|mermaid|mmd|pdf|png|rst|svg|toml|txt|webp|xml|ya?ml)$/i;
const WORKSPACE_IMAGE_EXTENSION_PATTERN = /\.(?:png|jpe?g|webp|gif|avif)$/i;

const ARTIFACT_PATH_RESOLVERS = [
  resolveAbsoluteArtifactPath,
  resolveListedArtifactPath,
  resolveAllowedUnlistedArtifactPath,
] as const;

const ARTIFACT_LINE_PARSERS: ArtifactLineParser[] = [
  parseSavedArtifactLine,
  parseAbsoluteArtifactLine,
];

function normalizeWorkspaceReference(value: string): string {
  return value
    .replace(/%60/gi, "`")
    .replace(/^[('"`【]+|[)'"`】,，。；：:!?]+$/g, "");
}

function looksLikeWorkspaceFileReference(value: string): boolean {
  if (!value.includes(".") || /^https?:\/\//i.test(value) || value.startsWith("/")) {
    return false;
  }
  return /[A-Za-z0-9]/.test(value);
}

export function createWorkspaceFileResolver(
  files: WorkspaceFileEntry[],
): ResolveWorkspaceFilePath {
  const paths = new Set<string>();
  const uniquePathByName = new Map<string, string | null>();

  for (const entry of files) {
    if (entry.is_dir) {
      continue;
    }
    paths.add(entry.path);
    uniquePathByName.set(
      entry.name,
      uniquePathByName.has(entry.name) ? null : entry.path,
    );
  }

  return (value) => {
    const normalized = normalizeWorkspaceReference(value);
    if (!looksLikeWorkspaceFileReference(normalized)) {
      return null;
    }
    return paths.has(normalized)
      ? normalized
      : uniquePathByName.get(normalized) ?? null;
  };
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
  return match?.groups?.relative ?? normalized.replace(/^\.\//, "");
}

export function resolveWorkspaceArtifactPath(
  path: string,
  resolveFilePath: ResolveWorkspaceFilePath,
  options: ResolveWorkspaceArtifactPathOptions = {},
): string | null {
  const context: ArtifactPathContext = {
    allowUnlistedRelativePath: options.allowUnlistedRelativePath ?? false,
    normalizedPath: normalizeWorkspaceReference(path).replace(/\\/g, "/"),
    resolveFilePath,
  };
  for (const resolver of ARTIFACT_PATH_RESOLVERS) {
    const resolvedPath = resolver(context);
    if (resolvedPath !== null) {
      return resolvedPath;
    }
  }
  return null;
}

function resolveAbsoluteArtifactPath(
  context: ArtifactPathContext,
): string | null {
  return WORKSPACE_ABSOLUTE_FILE_PATTERN.test(context.normalizedPath)
    ? clickableWorkspaceArtifactPath(context.normalizedPath)
    : null;
}

function resolveListedArtifactPath(
  context: ArtifactPathContext,
): string | null {
  return context.resolveFilePath(context.normalizedPath);
}

function resolveAllowedUnlistedArtifactPath(
  context: ArtifactPathContext,
): string | null {
  if (!context.allowUnlistedRelativePath) {
    return null;
  }
  const isAllowed = isWorkspaceRelativeArtifactPath(context.normalizedPath)
    || (
      isWorkspaceImagePath(context.normalizedPath)
      && looksLikeWorkspaceFileReference(context.normalizedPath)
    );
  return isAllowed ? context.normalizedPath.replace(/^\.\//, "") : null;
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
  return looksLikeWorkspaceFileReference(normalized)
    && normalized.includes("/")
    && WORKSPACE_ARTIFACT_EXTENSION_PATTERN.test(normalized);
}

export function splitMarkdownFileArtifacts(
  content: string,
  resolveFilePath: ResolveWorkspaceFilePath,
): MarkdownContentSegment[] {
  const accumulator: MarkdownSegmentAccumulator = {
    pendingText: [],
    segments: [],
  };

  for (const line of content.split("\n")) {
    const parsedLine = parseArtifactLine(line, resolveFilePath);
    if (parsedLine) {
      appendArtifactLine(accumulator, parsedLine);
    } else {
      accumulator.pendingText.push(line);
    }
  }

  flushPendingText(accumulator);
  return accumulator.segments.length > 0
    ? accumulator.segments
    : [{ type: "text", text: content }];
}

function parseArtifactLine(
  line: string,
  resolveFilePath: ResolveWorkspaceFilePath,
): ParsedArtifactLine | null {
  for (const parser of ARTIFACT_LINE_PARSERS) {
    const parsedLine = parser(line, resolveFilePath);
    if (parsedLine !== null) {
      return parsedLine;
    }
  }
  return null;
}

function parseSavedArtifactLine(
  line: string,
  resolveFilePath: ResolveWorkspaceFilePath,
): ParsedArtifactLine | null {
  const match = SAVED_FILE_LINE_PATTERN.exec(line.trim());
  const path = match?.groups?.path;
  if (!path) {
    return null;
  }
  return createParsedArtifactLine({
    label: normalizeArtifactLabel(match.groups?.prefix ?? ""),
    leadingText: null,
    path,
    resolveFilePath,
    trailingText: meaningfulArtifactText(match.groups?.suffix),
  });
}

function parseAbsoluteArtifactLine(
  line: string,
  resolveFilePath: ResolveWorkspaceFilePath,
): ParsedArtifactLine | null {
  const normalizedLine = line.trim();
  const match = WORKSPACE_ABSOLUTE_FILE_PATTERN.exec(normalizedLine);
  const path = match?.groups?.path;
  if (!match || !path) {
    return null;
  }
  const pathEnd = match.index + path.length;
  return createParsedArtifactLine({
    label: "文件",
    leadingText: meaningfulArtifactText(normalizedLine.slice(0, match.index)),
    path,
    resolveFilePath,
    trailingText: meaningfulArtifactText(normalizedLine.slice(pathEnd)),
  });
}

function createParsedArtifactLine({
  label,
  leadingText,
  path,
  resolveFilePath,
  trailingText,
}: {
  label: string;
  leadingText: string | null;
  path: string;
  resolveFilePath: ResolveWorkspaceFilePath;
  trailingText: string | null;
}): ParsedArtifactLine | null {
  const resolvedPath = resolveWorkspaceArtifactPath(path, resolveFilePath, {
    allowUnlistedRelativePath: true,
  });
  if (!resolvedPath) {
    return null;
  }
  return {
    artifact: {
      type: "file_artifact",
      display_path: displayWorkspaceArtifactPath(path),
      label,
      path: resolvedPath,
    },
    leadingText,
    trailingText,
  };
}

function meaningfulArtifactText(value: string | undefined): string | null {
  const normalized = value?.trim() ?? "";
  return /[\p{L}\p{N}]/u.test(normalized) ? normalized : null;
}

function appendArtifactLine(
  accumulator: MarkdownSegmentAccumulator,
  parsedLine: ParsedArtifactLine,
): void {
  if (parsedLine.leadingText) {
    accumulator.pendingText.push(parsedLine.leadingText);
  }
  flushPendingText(accumulator);
  accumulator.segments.push(parsedLine.artifact);
  if (parsedLine.trailingText) {
    accumulator.pendingText.push(parsedLine.trailingText);
  }
}

function flushPendingText(accumulator: MarkdownSegmentAccumulator): void {
  if (accumulator.pendingText.length === 0) {
    return;
  }
  accumulator.segments.push({
    type: "text",
    text: accumulator.pendingText.join("\n"),
  });
  accumulator.pendingText.length = 0;
}
