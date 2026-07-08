import { readMarkdownFenceMarker } from "./markdown-fence";

type MarkdownStreamBlockState = "revealed" | "streaming";

export interface MarkdownStreamBlock {
  content: string;
  start_offset: number;
  state: MarkdownStreamBlockState;
}

interface MarkdownRawBlock {
  content: string;
  start_offset: number;
}

function getLinesWithEndings(content: string): string[] {
  return content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [];
}

function isBlankLine(line: string): boolean {
  return line.trim().length === 0;
}

function isStandaloneBlockLine(line: string): boolean {
  return /^ {0,3}#{1,6}\s+\S/.test(line) || /^ {0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/.test(line);
}

function splitMarkdownRawBlocks(content: string): MarkdownRawBlock[] {
  const blocks: MarkdownRawBlock[] = [];
  const buffer: string[] = [];
  let blockStartOffset = 0;
  let cursorOffset = 0;
  let openFence: { marker: "`" | "~"; length: number } | null = null;

  const flushBuffer = () => {
    if (buffer.length === 0) {
      blockStartOffset = cursorOffset;
      return;
    }

    blocks.push({
      content: buffer.join(""),
      start_offset: blockStartOffset,
    });
    buffer.length = 0;
    blockStartOffset = cursorOffset;
  };

  for (const line of getLinesWithEndings(content)) {
    const fenceMarker = readMarkdownFenceMarker(line);

    buffer.push(line);
    cursorOffset += line.length;

    if (openFence) {
      if (
        fenceMarker &&
        fenceMarker.marker === openFence.marker &&
        fenceMarker.length >= openFence.length
      ) {
        openFence = null;
        flushBuffer();
      }
      continue;
    }

    if (fenceMarker) {
      openFence = fenceMarker;
      continue;
    }

    if (isBlankLine(line) || (buffer.length === 1 && isStandaloneBlockLine(line))) {
      flushBuffer();
    }
  }

  flushBuffer();
  return blocks;
}

export function splitStreamingMarkdownBlocks(content: string): MarkdownStreamBlock[] {
  const rawBlocks = splitMarkdownRawBlocks(content);
  const tailIndex = rawBlocks.length - 1;

  return rawBlocks.map((block, index) => ({
    ...block,
    state: index === tailIndex ? "streaming" : "revealed",
  }));
}
