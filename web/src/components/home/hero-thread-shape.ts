"use client";

import { clamp, type BlobPoint } from "@/components/home/hero-blob-shape";

export const THREAD_STORAGE_KEY = "nexus-home-thread-bundles";

export const THREAD_POINT_KEYS = ["start", "control1", "control2", "end"] as const;

export type ThreadPointKey = (typeof THREAD_POINT_KEYS)[number];

export interface ThreadBundlePoints {
  control1: BlobPoint;
  control2: BlobPoint;
  end: BlobPoint;
  start: BlobPoint;
}

export interface ThreadBundleConfig extends ThreadBundlePoints {
  glowAlpha: number;
  glowWidth: number;
  groupAlpha: number;
  seed: number;
  strandCount: number;
}

export const DEFAULT_THREAD_BUNDLES: ThreadBundleConfig[] = [
  {
    control1: { x: -0.24, y: 0.36 },
    control2: { x: -0.08, y: 0.42 },
    end: { x: 0.02, y: 0.5 },
    glowAlpha: 0.12,
    glowWidth: 8,
    groupAlpha: 0.92,
    seed: 0.12,
    start: { x: -0.36, y: 0.62 },
    strandCount: 4,
  },
  {
    control1: { x: -0.18, y: 0.48 },
    control2: { x: -0.04, y: 0.54 },
    end: { x: 0.06, y: 0.6 },
    glowAlpha: 0.1,
    glowWidth: 7,
    groupAlpha: 0.84,
    seed: 0.18,
    start: { x: -0.34, y: 0.72 },
    strandCount: 4,
  },
  {
    control1: { x: 0.8, y: 0.04 },
    control2: { x: 1.02, y: 0.08 },
    end: { x: 0.95, y: 0.18 },
    glowAlpha: 0.1,
    glowWidth: 7,
    groupAlpha: 0.88,
    seed: 0.28,
    start: { x: 0.72, y: 0.02 },
    strandCount: 5,
  },
  {
    control1: { x: 0.92, y: 0.06 },
    control2: { x: 1.1, y: 0.14 },
    end: { x: 1.14, y: 0.22 },
    glowAlpha: 0.12,
    glowWidth: 8,
    groupAlpha: 0.9,
    seed: 0.44,
    start: { x: 0.82, y: 0.08 },
    strandCount: 5,
  },
  {
    control1: { x: 1.06, y: 0.28 },
    control2: { x: 1.06, y: 0.52 },
    end: { x: 0.94, y: 0.66 },
    glowAlpha: 0.11,
    glowWidth: 7,
    groupAlpha: 0.86,
    seed: 0.63,
    start: { x: 1.16, y: 0.24 },
    strandCount: 5,
  },
  {
    control1: { x: 1.04, y: 0.46 },
    control2: { x: 1.02, y: 0.68 },
    end: { x: 0.9, y: 0.8 },
    glowAlpha: 0.1,
    glowWidth: 7,
    groupAlpha: 0.84,
    seed: 0.8,
    start: { x: 1.12, y: 0.42 },
    strandCount: 4,
  },
  {
    control1: { x: 0.3, y: 1.02 },
    control2: { x: 0.32, y: 0.92 },
    end: { x: 0.38, y: 0.84 },
    glowAlpha: 0.12,
    glowWidth: 8,
    groupAlpha: 0.88,
    seed: 0.92,
    start: { x: 0.24, y: 1.24 },
    strandCount: 5,
  },
  {
    control1: { x: 0.46, y: 1.06 },
    control2: { x: 0.46, y: 0.94 },
    end: { x: 0.5, y: 0.84 },
    glowAlpha: 0.1,
    glowWidth: 7,
    groupAlpha: 0.82,
    seed: 1.04,
    start: { x: 0.44, y: 1.26 },
    strandCount: 4,
  },
  {
    control1: { x: 0.6, y: 1.04 },
    control2: { x: 0.64, y: 0.92 },
    end: { x: 0.66, y: 0.82 },
    glowAlpha: 0.1,
    glowWidth: 7,
    groupAlpha: 0.84,
    seed: 1.18,
    start: { x: 0.62, y: 1.24 },
    strandCount: 4,
  },
];

function clampThreadPoint(point: BlobPoint): BlobPoint {
  return {
    x: clamp(point.x, -0.6, 1.4),
    y: clamp(point.y, -0.2, 1.4),
  };
}

function sanitizeThreadPoint(point: unknown): BlobPoint | null {
  if (!point || typeof point !== "object") {
    return null;
  }

  const candidate = point as Partial<BlobPoint>;
  if (!Number.isFinite(candidate.x) || !Number.isFinite(candidate.y)) {
    return null;
  }

  return clampThreadPoint({
    x: candidate.x!,
    y: candidate.y!,
  });
}

export function parseThreadBundles(raw: string | null): ThreadBundleConfig[] | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Array<Partial<ThreadBundlePoints>>;
    if (!Array.isArray(parsed) || parsed.length !== DEFAULT_THREAD_BUNDLES.length) {
      return null;
    }

    const nextBundles = parsed.map((bundle, index) => {
      const start = sanitizeThreadPoint(bundle?.start);
      const control1 = sanitizeThreadPoint(bundle?.control1);
      const control2 = sanitizeThreadPoint(bundle?.control2);
      const end = sanitizeThreadPoint(bundle?.end);
      if (!start || !control1 || !control2 || !end) {
        return null;
      }

      return {
        ...DEFAULT_THREAD_BUNDLES[index],
        control1,
        control2,
        end,
        start,
      };
    });

    if (nextBundles.some((bundle) => bundle === null)) {
      return null;
    }

    return nextBundles as ThreadBundleConfig[];
  } catch {
    return null;
  }
}

export function serializeThreadBundles(bundles: ThreadBundleConfig[]): string {
  return JSON.stringify(
    bundles.map((bundle) => ({
      control1: bundle.control1,
      control2: bundle.control2,
      end: bundle.end,
      start: bundle.start,
    })),
    null,
    2,
  );
}
