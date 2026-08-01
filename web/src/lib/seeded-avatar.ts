export interface SeededAvatarAppearance {
  backgroundColor: string;
  foregroundColor: string;
  pathData: string;
}

const SEEDED_AVATAR_PALETTES = [
  { backgroundColor: "#E8F39A", foregroundColor: "#182109" },
  { backgroundColor: "#DCEAFF", foregroundColor: "#17365F" },
  { backgroundColor: "#FFE1C4", foregroundColor: "#542B0F" },
  { backgroundColor: "#E8DFFF", foregroundColor: "#382560" },
  { backgroundColor: "#D8F0E5", foregroundColor: "#173E2D" },
  { backgroundColor: "#FFDDE3", foregroundColor: "#57212D" },
  { backgroundColor: "#F8E6A8", foregroundColor: "#49370E" },
  { backgroundColor: "#D7EFF2", foregroundColor: "#173B41" },
] as const;

interface CurvePoint {
  x: number;
  y: number;
}

type CurvePointBuilder = (
  progress: number,
  randomValues: readonly number[],
) => CurvePoint;
type SeededRandom = () => number;

const CURVE_STEP_COUNT = 144;
const CURVE_TARGET_SIZE = 68;
const LISSAJOUS_FREQUENCY_PAIRS = [
  [2, 3],
  [2, 5],
  [3, 4],
  [3, 5],
  [4, 5],
] as const;
const TAU = Math.PI * 2;

/** 中文注释：使用稳定散列代替随机数，确保同一标识在所有页面保持同一头像。 */
function hashSeed(value: string): number {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function createSeededRandom(seed: string): SeededRandom {
  let state = hashSeed(seed) || 0x6d2b79f5;
  return () => {
    state += 0x6d2b79f5;
    let value = state;
    value = Math.imul(value ^ (value >>> 15), value | 1);
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
  };
}

function randomInteger(
  random: SeededRandom,
  minimum: number,
  maximum: number,
): number {
  return minimum + Math.floor(random() * (maximum - minimum + 1));
}

function buildOrbitPoint(
  progress: number,
  randomValues: readonly number[],
): CurvePoint {
  const t = progress * TAU;
  const primaryFrequency = 2 + Math.floor(randomValues[0] * 5);
  const secondaryFrequency = 2 + Math.floor(randomValues[1] * 4);
  const radialAmplitude = 0.12 + randomValues[2] * 0.12;
  const harmonicAmplitude = 0.08 + randomValues[3] * 0.08;
  const primaryPhase = randomValues[4] * TAU;
  const secondaryPhase = randomValues[5] * TAU;
  const radius = 1 + radialAmplitude * Math.cos(
    primaryFrequency * t + primaryPhase,
  );
  return {
    x: radius * Math.cos(t)
      + harmonicAmplitude * Math.cos(secondaryFrequency * t + secondaryPhase),
    y: radius * Math.sin(t)
      - harmonicAmplitude * Math.sin(secondaryFrequency * t + secondaryPhase),
  };
}

function buildLissajousPoint(
  progress: number,
  randomValues: readonly number[],
): CurvePoint {
  const t = progress * TAU;
  const frequencies = LISSAJOUS_FREQUENCY_PAIRS[
    Math.floor(randomValues[0] * LISSAJOUS_FREQUENCY_PAIRS.length)
  ];
  const [xFrequency, yFrequency] = frequencies;
  const phase = 0.32 + randomValues[2] * 0.72;
  const harmonicAmplitude = 0.025 + randomValues[3] * 0.045;
  const harmonicFrequency = 2 + Math.floor(randomValues[4] * 2);
  const harmonicPhase = randomValues[5] * TAU;
  return {
    x: Math.sin(xFrequency * t + phase)
      + harmonicAmplitude * Math.sin(harmonicFrequency * t + harmonicPhase),
    y: Math.sin(yFrequency * t)
      + harmonicAmplitude * Math.cos((harmonicFrequency + 1) * t + harmonicPhase),
  };
}

function buildRosePoint(
  progress: number,
  randomValues: readonly number[],
): CurvePoint {
  const t = progress * TAU;
  const petalCount = 2 + Math.floor(randomValues[0] * 4);
  const warpFrequency = 2 + Math.floor(randomValues[1] * 3);
  const warpAmplitude = 0.04 + randomValues[2] * 0.1;
  const warpPhase = randomValues[3] * TAU;
  const radius = Math.cos(petalCount * t)
    * (1 + warpAmplitude * Math.cos(warpFrequency * t + warpPhase));
  return {
    x: radius * Math.cos(t),
    y: radius * Math.sin(t),
  };
}

const CURVE_POINT_BUILDERS: readonly CurvePointBuilder[] = [
  buildOrbitPoint,
  buildLissajousPoint,
  buildRosePoint,
];

function rotatePoint(point: CurvePoint, angle: number): CurvePoint {
  const cosine = Math.cos(angle);
  const sine = Math.sin(angle);
  return {
    x: point.x * cosine - point.y * sine,
    y: point.x * sine + point.y * cosine,
  };
}

function normalizeCurve(points: readonly CurvePoint[]): CurvePoint[] {
  const bounds = points.reduce(
    (result, point) => ({
      maximumX: Math.max(result.maximumX, point.x),
      maximumY: Math.max(result.maximumY, point.y),
      minimumX: Math.min(result.minimumX, point.x),
      minimumY: Math.min(result.minimumY, point.y),
    }),
    {
      maximumX: Number.NEGATIVE_INFINITY,
      maximumY: Number.NEGATIVE_INFINITY,
      minimumX: Number.POSITIVE_INFINITY,
      minimumY: Number.POSITIVE_INFINITY,
    },
  );
  const width = bounds.maximumX - bounds.minimumX;
  const height = bounds.maximumY - bounds.minimumY;
  const scale = CURVE_TARGET_SIZE / Math.max(width, height, 1);
  const centerX = (bounds.maximumX + bounds.minimumX) / 2;
  const centerY = (bounds.maximumY + bounds.minimumY) / 2;
  return points.map((point) => ({
    x: 50 + (point.x - centerX) * scale,
    y: 50 + (point.y - centerY) * scale,
  }));
}

function buildCurvePath(seed: string): string {
  const random = createSeededRandom(`curve:${seed}`);
  const buildPoint = CURVE_POINT_BUILDERS[
    randomInteger(random, 0, CURVE_POINT_BUILDERS.length - 1)
  ];
  const randomValues = Array.from({ length: 6 }, () => random());
  const rotation = random() * TAU;
  const points = Array.from({ length: CURVE_STEP_COUNT + 1 }, (_, index) => {
    const progress = index / CURVE_STEP_COUNT;
    const point = buildPoint(progress, randomValues);
    return rotatePoint(point, rotation);
  });
  return normalizeCurve(points)
    .map((point, index) => (
      `${index === 0 ? "M" : "L"}${point.x.toFixed(2)} ${point.y.toFixed(2)}`
    ))
    .join(" ");
}

export function getSeededAvatarAppearance(
  seed: string,
): SeededAvatarAppearance {
  const normalizedSeed = seed.trim().toLowerCase() || "nexus";
  const palette = SEEDED_AVATAR_PALETTES[
    hashSeed(`palette:${normalizedSeed}`) % SEEDED_AVATAR_PALETTES.length
  ];
  return {
    backgroundColor: palette.backgroundColor,
    foregroundColor: palette.foregroundColor,
    pathData: buildCurvePath(normalizedSeed),
  };
}
