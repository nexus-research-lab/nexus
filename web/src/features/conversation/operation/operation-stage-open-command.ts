/**
 * INPUT: A Bash command from the SDK, including Nexus stage redirect markers.
 * OUTPUT: The original display command and a validated URL or previewable file target.
 * POS: Shared command boundary for Terminal display and desktop open-intent routing.
 */

const STAGE_OPEN_REDIRECT_MARKER = "__NEXUS_STAGE_OPEN_V1__";

const PREVIEWABLE_EXTENSIONS = new Set([
  "html", "htm", "xhtml",
  "md", "mdx", "markdown", "txt", "log",
  "json", "yaml", "yml", "toml", "xml",
  "c", "cc", "cpp", "h", "hpp", "go", "rs", "py",
  "js", "jsx", "ts", "tsx", "css", "scss", "less",
  "java", "kt", "swift", "sh", "bash", "zsh", "sql",
  "vue", "svelte", "graphql", "ipynb", "ps1", "lua", "dart", "cs", "fs", "fsx",
  "doc", "docx", "rtf", "odt", "pdf",
  "csv", "tsv", "xls", "xlsx", "ods", "ppt", "pptx", "odp",
  "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif", "tif", "tiff",
]);

export interface StageOpenCommand {
  command: string;
  target: string;
  url: string | null;
}

export function readStageOpenCommand(command: string): StageOpenCommand | null {
  const redirected = decode_stage_open_redirect(command);
  if (redirected) {
    return {
      command: redirected.command,
      target: redirected.target,
      url: looks_like_web_url(redirected.target) ? redirected.target : null,
    };
  }

  const target = extract_open_target(command);
  if (!target || (!looks_like_web_url(target) && !looks_like_preview_target(target))) {
    return null;
  }
  const normalized_target = target.replace(/^file:\/\//i, "");
  return {
    command,
    target: normalized_target,
    url: looks_like_web_url(normalized_target) ? normalized_target : null,
  };
}

export function readStageDisplayCommand(command: string): string {
  return decode_stage_open_redirect(command)?.command ?? command;
}

function decode_stage_open_redirect(command: string): { command: string; target: string } | null {
  const marker_index = command.indexOf(STAGE_OPEN_REDIRECT_MARKER);
  if (marker_index < 0) {
    return null;
  }
  const encoded = command.slice(marker_index + STAGE_OPEN_REDIRECT_MARKER.length).trim();
  if (!encoded) {
    return null;
  }
  try {
    const base64 = encoded.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64.padEnd(Math.ceil(base64.length / 4) * 4, "=");
    const bytes = Uint8Array.from(atob(padded), (character) => character.charCodeAt(0));
    const value = JSON.parse(new TextDecoder().decode(bytes)) as unknown;
    if (!value || typeof value !== "object") {
      return null;
    }
    const record = value as Record<string, unknown>;
    const original_command = typeof record.command === "string" ? record.command.trim() : "";
    const target = typeof record.target === "string" ? record.target.trim() : "";
    return original_command && target
      ? { command: original_command, target }
      : null;
  } catch {
    return null;
  }
}

function extract_open_target(command: string): string | null {
  const tokens = split_simple_shell_command(command);
  if (!tokens || tokens.length < 2) {
    return null;
  }
  const tool = tokens[0].split(/[\\/]/).at(-1)?.toLowerCase();
  if (tool !== "open" && tool !== "xdg-open" && tool !== "start") {
    return null;
  }

  let target = "";
  let skip_next = false;
  let options_ended = false;
  for (const argument of tokens.slice(1)) {
    if (skip_next) {
      skip_next = false;
      continue;
    }
    if (!options_ended && argument === "--") {
      options_ended = true;
      continue;
    }
    if (!options_ended && argument.startsWith("-") && argument !== "-") {
      if (tool === "open" && ["-a", "-b", "--app", "--bundle-id"].includes(argument)) {
        skip_next = true;
      }
      continue;
    }
    if (argument.trim()) {
      target = argument.trim();
    }
  }
  return target || null;
}

function split_simple_shell_command(command: string): string[] | null {
  const normalized = command.trim();
  if (!normalized) {
    return null;
  }
  const tokens: string[] = [];
  let token = "";
  let quote: "'" | '"' | null = null;
  let escaped = false;
  let started = false;
  const flush = () => {
    if (started) {
      tokens.push(token);
      token = "";
      started = false;
    }
  };

  for (const character of normalized) {
    if (escaped) {
      token += character;
      started = true;
      escaped = false;
      continue;
    }
    if (character === "\\" && quote !== "'") {
      escaped = true;
      started = true;
      continue;
    }
    if (quote) {
      if (character === quote) {
        quote = null;
        started = true;
      } else {
        token += character;
        started = true;
      }
      continue;
    }
    if (character === "'" || character === '"') {
      quote = character;
      started = true;
      continue;
    }
    if ([";", "&", "|", "\n", "\r"].includes(character)) {
      return null;
    }
    if (character === " " || character === "\t") {
      flush();
      continue;
    }
    token += character;
    started = true;
  }
  if (escaped || quote) {
    return null;
  }
  flush();
  return tokens;
}

function looks_like_preview_target(value: string): boolean {
  const path = value.split(/[?#]/, 1)[0] ?? "";
  const filename = path.split(/[\\/]/).at(-1) ?? path;
  const extension = filename.includes(".") ? filename.slice(filename.lastIndexOf(".") + 1).toLowerCase() : "";
  return PREVIEWABLE_EXTENSIONS.has(extension);
}

function looks_like_web_url(value: string): boolean {
  return /^https?:\/\//i.test(value.trim());
}
