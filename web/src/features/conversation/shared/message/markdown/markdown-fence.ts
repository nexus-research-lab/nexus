export interface MarkdownFenceMarker {
  language: string;
  length: number;
  marker: "`" | "~";
}

export interface MarkdownOpenFence extends MarkdownFenceMarker {
  start_offset: number;
}

export function read_markdown_fence_marker(line: string): MarkdownFenceMarker | null {
  const match = /^ {0,3}(`{3,}|~{3,})(?<info>[^\r\n]*)$/.exec(line.trimEnd());
  if (!match) {
    return null;
  }

  const marker_text = match[1];
  return {
    language: match.groups?.info.trim().split(/\s+/, 1)[0]?.toLowerCase() ?? "",
    length: marker_text.length,
    marker: marker_text[0] as "`" | "~",
  };
}

export function find_open_markdown_fence(content: string): MarkdownOpenFence | null {
  let open_fence: MarkdownOpenFence | null = null;
  let cursor_offset = 0;

  for (const line of content.match(/[^\n]*(?:\n|$)/g)?.filter((item) => item.length > 0) ?? []) {
    const marker = read_markdown_fence_marker(line);
    if (!marker) {
      cursor_offset += line.length;
      continue;
    }

    if (
      open_fence &&
      marker.marker === open_fence.marker &&
      marker.length >= open_fence.length
    ) {
      open_fence = null;
    } else if (!open_fence) {
      open_fence = {
        ...marker,
        start_offset: cursor_offset,
      };
    }

    cursor_offset += line.length;
  }

  return open_fence;
}

export function find_open_markdown_fence_language(content: string): string | null {
  return find_open_markdown_fence(content)?.language ?? null;
}
