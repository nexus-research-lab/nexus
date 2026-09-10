// INPUT: 原始 Markdown 和既有代码/链接保护判断。
// OUTPUT: 预处理应原样保留的显式公式范围，包含未闭合尾部。
// POS: 预处理保护边界；只阻止改写，真正的 Markdown 语法判定仍由 micromark 完成。

interface MathRange { start: number; end: number }

export function readMarkdownMathRanges(content: string, isProtected: (offset: number) => boolean): MathRange[] {
  const ranges: MathRange[] = [];
  let cursor = 0;
  while (cursor < content.length) {
    const opening = content.startsWith("\\(", cursor) ? "\\("
      : content.startsWith("\\[", cursor) ? "\\["
        : content[cursor] === "$" ? (content.slice(cursor).match(/^\$+/)?.[0] ?? "$") : null;
    if (!opening || isProtected(cursor)) {
      cursor += content[cursor] === "\\" ? 2 : 1;
      continue;
    }
    const start = cursor;
    const closing = opening === "\\(" ? "\\)" : opening === "\\[" ? "\\]" : opening;
    let closed = false;
    cursor += opening.length;
    while (cursor < content.length) {
      if (opening === "$" && (content.startsWith("\\(", cursor) || content.startsWith("\\[", cursor))) break;
      if (content.startsWith(closing, cursor)) { cursor += closing.length; closed = true; break; }
      // Inline math cannot own a new paragraph; block formulas may contain blank lines.
      if ((opening === "$" || opening === "\\(") && (content[cursor] === "\r" || content[cursor] === "\n") && /^\r?\n\s*\r?\n/.test(content.slice(cursor))) break;
      cursor += content[cursor] === "\\" ? 2 : 1;
    }
    // A lone dollar can be a price; it must not disable ordinary file linking after it.
    if (closed || opening !== "$") ranges.push({ start, end: cursor });
  }
  return ranges;
}

export function isInMarkdownMathRange(ranges: MathRange[], offset: number): boolean {
  return ranges.some((range) => range.start <= offset && offset < range.end);
}
