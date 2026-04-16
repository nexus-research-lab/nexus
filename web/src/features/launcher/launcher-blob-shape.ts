"use client";

import { BlobPoint } from "@/types/app/launcher";

export const OUTER_VIEWBOX_WIDTH = 1040;
export const OUTER_VIEWBOX_HEIGHT = 760;

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

export function create_closed_spline_path(points: BlobPoint[]): string {
  if (points.length < 3) {
    return "";
  }

  const size = points.length;
  const get_point = (index: number) => points[(index + size) % size];
  let path = `M ${points[0].x} ${points[0].y}`;

  for (let index = 0; index < size; index += 1) {
    const previous = get_point(index - 1);
    const current = get_point(index);
    const next = get_point(index + 1);
    const afterNext = get_point(index + 2);

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

export function create_inner_points(points: BlobPoint[], scaleX = 0.82, scaleY = 0.8): BlobPoint[] {
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

