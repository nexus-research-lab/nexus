"use client";

export interface BlobPoint {
  x: number;
  y: number;
}

export const OUTER_STORAGE_KEY = "nexus-home-blob-points";
export const INPUT_STORAGE_KEY = "nexus-home-input-blob-points";
export const OUTER_VIEWBOX_WIDTH = 1040;
export const OUTER_VIEWBOX_HEIGHT = 760;
export const INPUT_VIEWBOX_WIDTH = 760;
export const INPUT_VIEWBOX_HEIGHT = 180;

export const DEFAULT_OUTER_POINTS: BlobPoint[] = [
  { x: 293.780931122449, y: 205.3283155448619 },
  { x: 367.18367346938777, y: 84.94457811503896 },
  { x: 458.5484693877551, y: 49.95085035062825 },
  { x: 565.2761479591837, y: 22.771711189256997 },
  { x: 645.8463010204082, y: 114.94478376210746 },
  { x: 715.3565051020408, y: 185.96376498653012 },
  { x: 773.827487244898, y: 275.17510847882863 },
  { x: 803.8734056122448, y: 397.8407057807391 },
  { x: 790.579081632653, y: 538.8940300656014 },
  { x: 687.8973214285714, y: 644.6019700989162 },
  { x: 552.1973852040817, y: 642.8358730746294 },
  { x: 437.12914540816325, y: 676.3839019474777 },
  { x: 323.3003826530612, y: 691.294137002077 },
  { x: 281.3405612244898, y: 569.3943693832645 },
  { x: 244.78220663265307, y: 358.8537232401752 },
];

export const DEFAULT_INPUT_POINTS: BlobPoint[] = [
  { x: 69.84602864583333, y: 120.99609375 },
  { x: 78.61002604166667, y: 50.34179687500001 },
  { x: 183.78959147135419, y: 41.958487374441965 },
  { x: 288.0934143066406, y: 40.51613943917411 },
  { x: 398.5457000732422, y: 43.472115652901785 },
  { x: 530.5555369059246, y: 43.41666085379464 },
  { x: 649.5563151041667, y: 42.529296875 },
  { x: 688.2737630208334, y: 60.703125 },
  { x: 688.7376302083334, y: 130 },
  { x: 642.4973195393881, y: 133.87924194335938 },
  { x: 498.83542887369794, y: 137.03404017857142 },
  { x: 342.8408559163411, y: 137.14102608816964 },
  { x: 226.41289774576822, y: 135.77793666294642 },
  { x: 123.987060546875, y: 139.5703125 },
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
