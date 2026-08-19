/**
 * INPUT: 任意 Unicode 流式正文。
 * OUTPUT: 不拆分 emoji ZWJ、肤色修饰符或组合附标的展示字符单元。
 * POS: 流式 backlog 的唯一字符边界；无 Intl.Segmenter 时退回 code point。
 */

const graphemeSegmenter = (
  typeof Intl !== "undefined"
  && typeof Intl.Segmenter === "function"
)
  ? new Intl.Segmenter(undefined, { granularity: "grapheme" })
  : null;

export function splitStreamingTextUnits(value: string): string[] {
  if (graphemeSegmenter === null) {
    return Array.from(value);
  }
  return Array.from(
    graphemeSegmenter.segment(value),
    ({ segment }) => segment,
  );
}

export interface AppendStreamingTextUnitsResult {
  appendedCount: number;
  replacedTrailingUnit: boolean;
}

export function appendStreamingTextUnits(
  target: string[],
  value: string,
): AppendStreamingTextUnitsResult {
  if (!value) {
    return { appendedCount: 0, replacedTrailingUnit: false };
  }

  const previousCount = target.length;
  const trailingUnit = target.pop() ?? "";
  const nextTrailingUnits = splitStreamingTextUnits(trailingUnit + value);
  for (const unit of nextTrailingUnits) {
    target.push(unit);
  }
  return {
    appendedCount: Math.max(0, target.length - previousCount),
    replacedTrailingUnit: (
      trailingUnit.length > 0
      && nextTrailingUnits[0] !== trailingUnit
    ),
  };
}

export function joinStreamingTextPrefix(
  units: string[],
  count: number,
): string {
  return units
    .slice(0, Math.max(0, Math.floor(count)))
    .join("");
}
