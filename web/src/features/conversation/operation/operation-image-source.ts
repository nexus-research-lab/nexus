/**
 * INPUT: An exact nxs ViewImage operation event.
 * OUTPUT: A classified image source with a safe Stage title and optional workspace target.
 * POS: ViewImage trust boundary; ephemeral references and URLs never masquerade as workspace files.
 */
import type { NexusOperationEvent } from "./operation-types";

export type OperationImageSourceKind =
  | "workspace"
  | "remote"
  | "inline"
  | "attachment"
  | "unavailable";

export interface OperationImageSource {
  kind: OperationImageSourceKind;
  source: string;
  target: string | null;
  title: string;
}

export function resolveOperationImageSource(
  event: NexusOperationEvent,
): OperationImageSource | null {
  if (event.tool_name !== "ViewImage") {
    return null;
  }
  const input_source = event.input_preview?.source;
  const source = typeof input_source === "string"
    ? input_source.trim()
    : event.target?.trim() ?? "";
  if (!source) {
    return null;
  }

  if (/^https?:\/\//i.test(source)) {
    return {
      kind: "remote",
      source,
      target: null,
      title: remote_image_title(source),
    };
  }
  if (/^data:image\/(?:png|jpe?g|gif|webp)(?:;[^,]*)?,/i.test(source)) {
    return { kind: "inline", source, target: null, title: "内联图片" };
  }
  if (/^data:/i.test(source)) {
    return { kind: "unavailable", source, target: null, title: "图像来源" };
  }
  if (/^nexus-image:\/\//i.test(source)) {
    return { kind: "attachment", source, target: null, title: "会话图片" };
  }

  const file_target = file_url_target(source);
  if (file_target) {
    return {
      kind: "workspace",
      source,
      target: file_target,
      title: basename(file_target),
    };
  }
  if (/^[a-z][a-z0-9+.-]*:/i.test(source) && !/^[a-z]:[\\/]/i.test(source)) {
    return { kind: "unavailable", source, target: null, title: "图像来源" };
  }

  const target = source.replace(/\\/g, "/").replace(/^\.\//, "");
  return {
    kind: "workspace",
    source,
    target,
    title: basename(target),
  };
}

function file_url_target(source: string): string | null {
  if (!/^file:\/\//i.test(source)) {
    return null;
  }
  try {
    const parsed = new URL(source);
    if (parsed.hostname && parsed.hostname !== "localhost") {
      return null;
    }
    const decoded = decodeURIComponent(parsed.pathname).replace(/\\/g, "/");
    return /^\/[a-z]:\//i.test(decoded) ? decoded.slice(1) : decoded;
  } catch {
    return source.replace(/^file:\/\//i, "");
  }
}

function remote_image_title(source: string): string {
  try {
    const url = new URL(source);
    const file_name = basename(decodeURIComponent(url.pathname));
    return file_name || url.hostname || "远程图片";
  } catch {
    return "远程图片";
  }
}

function basename(value: string): string {
  return value.split(/[\\/]/).filter(Boolean).at(-1) ?? value;
}
