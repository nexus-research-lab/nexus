export interface MarkdownFenceMarker {
  language: string;
  length: number;
  marker: "`" | "~";
}

interface MarkdownOpenFence extends MarkdownFenceMarker {
  start_offset: number;
}

export function readMarkdownFenceMarker(line: string): MarkdownFenceMarker | null {
  const match = /^ {0,3}(`{3,}|~{3,})(?<info>[^\r\n]*)$/.exec(line.trimEnd());
  if (!match) {
    return null;
  }

  const markerText = match[1];
  return {
    language: match.groups?.info.trim().split(/\s+/, 1)[0]?.toLowerCase() ?? "",
    length: markerText.length,
    marker: markerText[0] as "`" | "~",
  };
}

function findOpenMarkdownFence(content: string): MarkdownOpenFence | null {
  let openFence: MarkdownOpenFence | null = null;
  let cursorOffset = 0;

  for (const line of content.match(/[^\n]*(?:\n|$)/g)?.filter((item) => item.length > 0) ?? []) {
    const marker = readMarkdownFenceMarker(line);
    if (!marker) {
      cursorOffset += line.length;
      continue;
    }

    if (
      openFence &&
      marker.marker === openFence.marker &&
      marker.length >= openFence.length
    ) {
      openFence = null;
    } else if (!openFence) {
      openFence = {
        ...marker,
        start_offset: cursorOffset,
      };
    }

    cursorOffset += line.length;
  }

  return openFence;
}

export function findOpenMarkdownFenceLanguage(content: string): string | null {
  return findOpenMarkdownFence(content)?.language ?? null;
}
