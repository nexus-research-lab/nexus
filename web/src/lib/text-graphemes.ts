/**
 * INPUT: 任意 Unicode 展示文本。
 * OUTPUT: 完整 grapheme 列表；不支持 Intl.Segmenter 时退回 code point。
 * POS: 姓名缩写、文本动效和流式正文共用的无状态字符边界；不持有排版或时钟。
 */

const graphemeSegmenter = (
  typeof Intl !== "undefined"
  && typeof Intl.Segmenter === "function"
)
  ? new Intl.Segmenter(undefined, { granularity: "grapheme" })
  : null;

export function splitTextGraphemes(value: string): string[] {
  if (graphemeSegmenter === null) {
    return Array.from(value);
  }
  return Array.from(graphemeSegmenter.segment(value), ({ segment }) => segment);
}
