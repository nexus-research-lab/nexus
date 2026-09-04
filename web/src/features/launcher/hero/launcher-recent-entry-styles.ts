// INPUT: 最近 DM 入口的稳定键。
// OUTPUT: 使用全站语义色生成、跨重排稳定的 Launcher 身份点样式。
// POS: Launcher 最近入口的装饰性身份配方；不拥有按钮外形、状态或业务标签。

const LAUNCHER_RECENT_ENTRY_MARKER_CLASS_NAMES = [
  "border-[color:color-mix(in_srgb,var(--success)_34%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_24%,transparent)]",
  "border-[color:color-mix(in_srgb,var(--warning)_38%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_25%,transparent)]",
  "border-[color:color-mix(in_srgb,var(--primary)_30%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_20%,transparent)]",
] as const;

const MARKER_BASE_CLASS_NAME = "h-2.5 w-2.5 shrink-0 rounded-full border";

export function getLauncherRecentEntryMarkerClassName(entryKey: string): string {
  let hash = 0;
  for (let index = 0; index < entryKey.length; index += 1) {
    hash = (hash * 31 + entryKey.charCodeAt(index)) >>> 0;
  }
  const paletteClassName = LAUNCHER_RECENT_ENTRY_MARKER_CLASS_NAMES[
    hash % LAUNCHER_RECENT_ENTRY_MARKER_CLASS_NAMES.length
  ] ?? LAUNCHER_RECENT_ENTRY_MARKER_CLASS_NAMES[0];
  return `${MARKER_BASE_CLASS_NAME} ${paletteClassName}`;
}
