// INPUT: 持续追加的 Markdown，包括代码和显示公式边界。
// OUTPUT: 保留列表与公式完整性的稳定起点分块。
// POS: 流式渲染分块 owner；只识别不可分割边界，语法与公式内容仍归 core。

import { readMarkdownFenceMarker } from "../core/markdown-fence";

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

type MarkdownListKind = "ordered" | "unordered";

function getLinesWithEndings(content: string): string[] {
  return content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [];
}

function isBlankLine(line: string): boolean {
  return line.trim().length === 0;
}

function readMathFence(line: string): string {
  return line.replace(/^(?:[ \t]*>[ \t]?)+/, "").trim()
    .replace(/^(?:[-+*]|\d{1,9}[.)])[ \t]+/, "").trim();
}

function isStandaloneBlockLine(line: string): boolean {
  return /^ {0,3}#{1,6}\s+\S/.test(line) || /^ {0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/.test(line);
}

function readListKind(content: string): MarkdownListKind | null {
  const firstLine = getLinesWithEndings(content).find((line) => !isBlankLine(line));
  if (!firstLine || isStandaloneBlockLine(firstLine)) {
    return null;
  }
  if (/^ {0,3}\d{1,9}[.)](?:[ \t]+|$)/.test(firstLine)) {
    return "ordered";
  }
  if (/^ {0,3}[*+-](?:[ \t]+|$)/.test(firstLine)) {
    return "unordered";
  }
  return null;
}

function mergeAdjacentListBlocks(blocks: MarkdownRawBlock[]): MarkdownRawBlock[] {
  const merged: Array<MarkdownRawBlock & { list_kind: MarkdownListKind | null }> = [];
  for (const block of blocks) {
    const listKind = readListKind(block.content);
    const previous = merged.at(-1);
    if (listKind !== null && previous?.list_kind === listKind) {
      previous.content += block.content;
      continue;
    }
    merged.push({ ...block, list_kind: listKind });
  }
  return merged.map(({ list_kind: _listKind, ...block }) => block);
}

function splitMarkdownRawBlocks(content: string): MarkdownRawBlock[] {
  const blocks: MarkdownRawBlock[] = [];
  const buffer: string[] = [];
  let blockStartOffset = 0;
  let cursorOffset = 0;
  let openFence: { marker: "`" | "~"; length: number } | null = null;
  let openMath: string | null = null;

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

    if (openMath) {
      const closing = readMathFence(line);
      if (closing === openMath || (openMath.startsWith("$") && /^\${2,}$/.test(closing) && closing.length >= openMath.length)) openMath = null;
      continue;
    }

    if (fenceMarker) {
      openFence = fenceMarker;
      continue;
    }

    const mathFence = readMathFence(line);
    if ((mathFence.startsWith("\\[") && !mathFence.includes("\\]")) || /^\${2,}$/.test(mathFence)) {
      openMath = mathFence.startsWith("\\[") ? "\\]" : mathFence;
      continue;
    }

    if (isBlankLine(line) || (buffer.length === 1 && isStandaloneBlockLine(line))) {
      flushBuffer();
    }
  }

  flushBuffer();
  return mergeAdjacentListBlocks(blocks);
}

export function splitStreamingMarkdownBlocks(content: string): MarkdownStreamBlock[] {
  const rawBlocks = splitMarkdownRawBlocks(content);
  const tailIndex = rawBlocks.length - 1;

  return rawBlocks.map((block, index) => ({
    ...block,
    state: index === tailIndex ? "streaming" : "revealed",
  }));
}
