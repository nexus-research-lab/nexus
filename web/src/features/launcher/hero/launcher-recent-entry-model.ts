import type { CSSProperties } from "react";

import type { RecentLauncherEntry } from "../console/launcher-console-types";
import {
  isLauncherChipTruncated,
  truncateLauncherChipLabel,
} from "../console/launcher-console-helpers";

interface EntryTypePresentation {
  ariaPrefix: string;
  background: string;
  boxShadow: string;
  labelPrefix: string;
  textColor: string;
}

const ENTRY_TYPE_PRESENTATION: Record<
  RecentLauncherEntry["type"],
  EntryTypePresentation
> = {
  dm: {
    ariaPrefix: "私聊",
    background: "var(--launcher-agent-chip-background)",
    boxShadow: "inset 0 0 0 1px var(--launcher-agent-chip-border)",
    labelPrefix: "",
    textColor: "var(--launcher-agent-chip-text)",
  },
  room: {
    ariaPrefix: "房间",
    background: "var(--launcher-room-chip-background)",
    boxShadow: "inset 0 0 0 1px var(--launcher-room-chip-border)",
    labelPrefix: "#",
    textColor: "var(--launcher-room-chip-text)",
  },
};

const DM_MARKER_STYLES: CSSProperties[] = [
  { backgroundColor: "#bff0ca", border: "1px solid #7fe3a8" },
  { backgroundColor: "#ffd7b8", border: "1px solid #e3c6ad" },
];

export interface LauncherRecentEntryPresentation {
  ariaLabel: string;
  chipLabel: string;
  chipStyle: CSSProperties;
  delayMs: number;
  entry: RecentLauncherEntry;
  markerStyle: CSSProperties | null;
  tooltipLabel: string | null;
}

export function buildLauncherRecentEntryPresentation(
  entry: RecentLauncherEntry,
  index: number,
): LauncherRecentEntryPresentation {
  const typePresentation = ENTRY_TYPE_PRESENTATION[entry.type];
  const fullLabel = `${typePresentation.labelPrefix}${entry.label}`;
  return {
    ariaLabel: `${typePresentation.ariaPrefix} ${entry.label}`,
    chipLabel: `${typePresentation.labelPrefix}${truncateLauncherChipLabel(entry.label)}`,
    chipStyle: {
      background: typePresentation.background,
      boxShadow: typePresentation.boxShadow,
      color: typePresentation.textColor,
    },
    delayMs: 580 + index * 55,
    entry,
    markerStyle: entry.type === "dm"
      ? DM_MARKER_STYLES[Math.min(index, DM_MARKER_STYLES.length - 1)]
      : null,
    tooltipLabel: isLauncherChipTruncated(entry.label) ? fullLabel : null,
  };
}

export function getLauncherHandoffDelay(entryCount: number): number {
  return 580 + entryCount * 55;
}
