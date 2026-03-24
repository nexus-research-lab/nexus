"use client";

import { BlobPoint } from "@/types/launcher";

export const OUTER_STORAGE_KEY = "nexus-home-blob-points";
export const INPUT_STORAGE_KEY = "nexus-home-input-blob-points";
export const SIDE_PANEL_STORAGE_KEY = "nexus-home-side-panel-blob-points";
export const OUTER_VIEWBOX_WIDTH = 1040;
export const OUTER_VIEWBOX_HEIGHT = 760;
export const INPUT_VIEWBOX_WIDTH = 760;
export const INPUT_VIEWBOX_HEIGHT = 180;
export const SIDE_PANEL_VIEWBOX_WIDTH = 420;
export const SIDE_PANEL_VIEWBOX_HEIGHT = 620;

export const DEFAULT_OUTER_POINTS: BlobPoint[] = [
  {"x": 340.6696428571429, "y": 210.5098285236303},
  {"x": 391.74521683673464, "y": 128.5181932245922},
  {"x": 477.50542091836735, "y": 103.59807611877875},
  {"x": 558.2413903061224, "y": 79.05144291091594},
  {"x": 643.8109056122448, "y": 175.96654119615224},
  {"x": 707.3475765306123, "y": 238.65621079046426},
  {"x": 739.6941964285714, "y": 310.09493935591803},
  {"x": 758.6926020408163, "y": 436.7059807611878},
  {"x": 731.9049744897959, "y": 544.1580928481807},
  {"x": 659.3064413265306, "y": 595.4843161856963},
  {"x": 564.0283801020408, "y": 586.4809703053115},
  {"x": 470.9971301020408, "y": 621.2467586783772},
  {"x": 353.6654974489796, "y": 644.1405269761606},
  {"x": 309.02774234693874, "y": 509.2492680886658},
  {"x": 288.28411989795916, "y": 351.75826014219996}
];

export const DEFAULT_INPUT_POINTS: BlobPoint[] = [
  {"x": 68.83170572916666, "y": 137.900390625},
  {"x": 73.3095703125, "y": 43.251953125},
  {"x": 183.68522135416669, "y": 42.880859375},
  {"x": 326.5563151041667, "y": 35.517578125},
  {"x": 465.41341145833337, "y": 39.150390625},
  {"x": 587.0764973958334, "y": 26.787109375},
  {"x": 675.1370442708333, "y": 34.189453125},
  {"x": 705.1770833333334, "y": 70.029296875},
  {"x": 694.7679036458334, "y": 135.751953125},
  {"x": 647.9111328125, "y": 138.49609375},
  {"x": 557.494140625, "y": 136.572265625},
  {"x": 415.19824218749994, "y": 142.314453125},
  {"x": 265.30729166666663, "y": 147.03125},
  {"x": 138.7705078125, "y": 146.533203125}
];

export const DEFAULT_SIDE_PANEL_POINTS: BlobPoint[] = [
  {"x": 26.900390625, "y": 45.52920190274841},
  {"x": 69.7265625, "y": 17.50611125792812},
  {"x": 159.45703125, "y": 16},
  {"x": 267.392578125, "y": 16},
  {"x": 350.712890625, "y": 16},
  {"x": 400.048828125, "y": 36.08235332980973},
  {"x": 404, "y": 165.0561575052854},
  {"x": 404, "y": 314.50581395348837},
  {"x": 404, "y": 449.08116411205077},
  {"x": 404, "y": 579.8112116807611},
  {"x": 349.7109375, "y": 604},
  {"x": 222.59179687500003, "y": 604},
  {"x": 130.58203125, "y": 604},
  {"x": 36.45703125, "y": 593.2979320824525},
  {"x": 18.62109375, "y": 521.4814019556026},
  {"x": 20.841796875, "y": 401.7752378435518},
  {"x": 24.076171875, "y": 220.86680761099365}
];

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function createClosedSplinePath(points: BlobPoint[]): string {
  if (points.length < 3) {
    return "";
  }

  const size = points.length;
  const getPoint = (index: number) => points[(index + size) % size];
  let path = `M ${points[0].x} ${points[0].y}`;

  for (let index = 0; index < size; index += 1) {
    const previous = getPoint(index - 1);
    const current = getPoint(index);
    const next = getPoint(index + 1);
    const afterNext = getPoint(index + 2);

    const controlPoint1 = {
      x: current.x + (next.x - previous.x) / 6,
      y: current.y + (next.y - previous.y) / 6,
    };
    const controlPoint2 = {
      x: next.x - (afterNext.x - current.x) / 6,
      y: next.y - (afterNext.y - current.y) / 6,
    };

    path += ` C ${controlPoint1.x} ${controlPoint1.y}, ${controlPoint2.x} ${controlPoint2.y}, ${next.x} ${next.y}`;
  }

  return `${path} Z`;
}

export function createInnerPoints(points: BlobPoint[], scaleX = 0.82, scaleY = 0.8): BlobPoint[] {
  if (points.length === 0) {
    return points;
  }

  const bounds = points.reduce(
    (current, point) => ({
      minX: Math.min(current.minX, point.x),
      maxX: Math.max(current.maxX, point.x),
      minY: Math.min(current.minY, point.y),
      maxY: Math.max(current.maxY, point.y),
    }),
    {
      maxX: Number.NEGATIVE_INFINITY,
      maxY: Number.NEGATIVE_INFINITY,
      minX: Number.POSITIVE_INFINITY,
      minY: Number.POSITIVE_INFINITY,
    },
  );

  const centerX = (bounds.minX + bounds.maxX) / 2;
  const centerY = (bounds.minY + bounds.maxY) / 2;

  return points.map((point) => ({
    x: centerX + (point.x - centerX) * scaleX,
    y: centerY + (point.y - centerY) * scaleY,
  }));
}

function distanceToSegment(point: BlobPoint, start: BlobPoint, end: BlobPoint): number {
  const dx = end.x - start.x;
  const dy = end.y - start.y;

  if (dx === 0 && dy === 0) {
    return Math.hypot(point.x - start.x, point.y - start.y);
  }

  const projection = ((point.x - start.x) * dx + (point.y - start.y) * dy) / (dx * dx + dy * dy);
  const t = clamp(projection, 0, 1);
  const closestX = start.x + t * dx;
  const closestY = start.y + t * dy;

  return Math.hypot(point.x - closestX, point.y - closestY);
}

export function findNearestSegmentIndex(points: BlobPoint[], point: BlobPoint): number {
  let bestIndex = 0;
  let bestDistance = Number.POSITIVE_INFINITY;

  for (let index = 0; index < points.length; index += 1) {
    const start = points[index];
    const end = points[(index + 1) % points.length];
    const distance = distanceToSegment(point, start, end);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestIndex = index;
    }
  }

  return bestIndex;
}

export function parsePoints(
  raw: string | null,
  maxWidth: number,
  maxHeight: number,
): BlobPoint[] | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as BlobPoint[];
    if (!Array.isArray(parsed) || parsed.length < 6) {
      return null;
    }

    return parsed
      .filter((point) => Number.isFinite(point?.x) && Number.isFinite(point?.y))
      .map((point) => ({
        x: clamp(point.x, 16, maxWidth - 16),
        y: clamp(point.y, 16, maxHeight - 16),
      }));
  } catch {
    return null;
  }
}
