import type { SessionNavigationItem } from "./session-navigator-model";

export const RULER_TRACK_TOP_SAFE_INSET_PX = 56;
export const RULER_TRACK_BOTTOM_SAFE_INSET_PX = 24;

const RULER_TICK_SPACING_PX = 14;
const WAVE_RADIUS_TICKS = 4;
const USER_TICK_COLOR = "#5b7cfa";
const LIVE_TICK_COLOR = "#7c8cff";
const NEUTRAL_TICK_COLOR = "var(--text-muted)";
const ACTIVE_NEUTRAL_TICK_COLOR = "var(--text-strong)";
const AGENT_TICK_COLORS = [
  "#10b981",
  "#f59e0b",
  "#ec4899",
  "#06b6d4",
  "#8b5cf6",
  "#ef4444",
  "#84cc16",
  "#14b8a6",
];

export function getRulerTrackHeight(itemCount: number): number {
  return Math.max(RULER_TICK_SPACING_PX, itemCount * RULER_TICK_SPACING_PX);
}

export function getTickDisplayPercent(index: number, total: number): number {
  return total > 0 ? ((index + 0.5) / total) * 100 : 50;
}

export function buildTickBackground(item: SessionNavigationItem): string {
  const segments = buildTickSegments(item);
  if (segments.length === 1) {
    return segments[0];
  }
  const step = 100 / segments.length;
  const stops = segments.flatMap((color, index) => [
    `${color} ${index * step}%`,
    `${color} ${(index + 1) * step}%`,
  ]);
  return `linear-gradient(90deg, ${stops.join(", ")})`;
}

export function buildTickVisual(
  item: SessionNavigationItem,
  activeRoundId: string | null,
  previewIndex: number | null,
  previewRoundId: string | null,
) {
  const hasPreview = previewIndex !== null;
  const isActive = item.roundId === activeRoundId;
  const isPreviewed = item.roundId === previewRoundId;
  const wave = hasPreview
    ? smoothWave(Math.abs(item.index - previewIndex))
    : 0;
  let background = NEUTRAL_TICK_COLOR;
  if (isPreviewed) {
    background = buildTickBackground(item);
  } else if (!hasPreview && isActive) {
    background = ACTIVE_NEUTRAL_TICK_COLOR;
  }
  let opacity = 0.58;
  if (hasPreview) {
    opacity = tickOpacity(wave);
  } else if (isActive) {
    opacity = 0.9;
  }

  return {
    background,
    filter: isPreviewed ? "saturate(1.18)" : undefined,
    opacity,
    width: hasPreview ? tickWidth(wave) : 5,
  };
}

export function formatSpeakerSummary(
  item: SessionNavigationItem,
  agentNameMap?: Record<string, string>,
): string {
  const speakers = item.hasUserMessage ? ["用户"] : [];
  speakers.push(
    ...item.agentIds.map(
      (agentId) => agentNameMap?.[agentId] || `Agent ${agentId.slice(0, 6)}`,
    ),
  );
  return speakers.join(" · ") || "未加载";
}

function buildTickSegments(item: SessionNavigationItem): string[] {
  const segments = item.hasUserMessage ? [USER_TICK_COLOR] : [];
  segments.push(...item.agentIds.slice(0, 4).map(getAgentTickColor));
  if (segments.length === 0) {
    segments.push(item.isLive ? LIVE_TICK_COLOR : NEUTRAL_TICK_COLOR);
  }
  return segments;
}

function getAgentTickColor(agentId: string): string {
  return AGENT_TICK_COLORS[hashText(agentId) % AGENT_TICK_COLORS.length];
}

function hashText(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function smoothWave(distanceTicks: number): number {
  const normalized = Math.max(0, 1 - distanceTicks / WAVE_RADIUS_TICKS);
  return normalized * normalized * (3 - 2 * normalized);
}

function tickWidth(wave: number): number {
  return Math.round(5 + wave * 11);
}

function tickOpacity(wave: number): number {
  return Math.min(1, 0.48 + wave * 0.42);
}
