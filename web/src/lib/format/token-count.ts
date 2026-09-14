// 摘要最多保留一位小数，精确数值由明细或悬浮提示展示。
export function formatTokens(tokens: number, locale = "en"): string {
  const units: [number, string][] = locale.startsWith("zh")
    ? [[1e12, "万亿"], [1e8, "亿"], [1e6, "百万"], [1e4, "万"], [1e3, "千"]]
    : [[1e12, "T"], [1e9, "B"], [1e6, "M"], [1e3, "K"]];
  const unit = units.find(([scale]) => Math.abs(tokens) >= scale);
  return unit ? `${Number((tokens / unit[0]).toFixed(1))}${unit[1]}` : String(tokens);
}
