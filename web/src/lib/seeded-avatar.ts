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

interface CurveParameters {
  detailScale: number;
  values: readonly number[];
}

type CurvePointBuilder = (
  progress: number,
  parameters: CurveParameters,
) => CurvePoint;
type SeededRandom = () => number;

const CURVE_STEP_COUNT = 192;
const CURVE_TARGET_RADIUS = 34;
const SEEDED_AVATAR_DATA_URL_CACHE_LIMIT = 256;
const seededAvatarDataUrlCache = new Map<string, string>();
const LISSAJOUS_FREQUENCY_PAIRS = [
  [2, 3],
  [2, 5],
  [3, 4],
  [3, 5],
  [4, 5],
  [5, 6],
] as const;
const TAU = Math.PI * 2;

function normalizeAvatarSeed(seed: string): string {
  return seed.trim().toLowerCase() || "nexus";
}

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

function integerFromUnit(value: number, minimum: number, maximum: number): number {
  return minimum + Math.floor(value * (maximum - minimum + 1));
}

function polarPoint(t: number, radius: number): CurvePoint {
  return {
    x: Math.cos(t) * radius,
    y: Math.sin(t) * radius,
  };
}

/** 中文注释：径向起伏只改变同阶花瓣，不叠加会把视觉重心推离原点的杂波。 */
function buildRadialPetalPoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const petalCount = integerFromUnit(parameters.values[0], 4, 8);
  const detailAmplitude = 0.28 + parameters.values[1] * 0.14;
  const radius = 1
    - detailAmplitude * parameters.detailScale * Math.cos(petalCount * t);
  return polarPoint(t, radius);
}

/** 中文注释：两个同余谐波天然共享旋转阶数，可形成闭合且居中的轨道花纹。 */
function buildHarmonicOrbitPoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const rotationalOrder = integerFromUnit(parameters.values[0], 3, 7);
  const harmonicFrequency = rotationalOrder + 1;
  const detailAmplitude = (0.26 + parameters.values[1] * 0.16)
    * parameters.detailScale;
  return {
    x: Math.cos(t) - detailAmplitude * Math.cos(harmonicFrequency * t),
    y: Math.sin(t) - detailAmplitude * Math.sin(harmonicFrequency * t),
  };
}

function buildRosePoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const petalFrequency = integerFromUnit(parameters.values[0], 2, 6);
  const wave = Math.cos(petalFrequency * t);
  const fullness = 0.72 + parameters.detailScale * 0.62;
  const radius = Math.sign(wave) * Math.abs(wave) ** fullness;
  return polarPoint(t, radius);
}

function buildLissajousPoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const frequencies = LISSAJOUS_FREQUENCY_PAIRS[
    integerFromUnit(
      parameters.values[0],
      0,
      LISSAJOUS_FREQUENCY_PAIRS.length - 1,
    )
  ];
  const [xFrequency, yFrequency] = frequencies;
  const phase = parameters.values[1] < 0.5 ? 0 : Math.PI / 2;
  const yScale = 0.82 + parameters.detailScale * 0.18;
  return {
    x: Math.sin(xFrequency * t + phase),
    y: Math.sin(yFrequency * t) * yScale,
  };
}

function buildLemniscatePoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const sine = Math.sin(t);
  const cosine = Math.cos(t);
  const denominator = 1
    + (0.72 + parameters.detailScale * 0.56) * sine ** 2;
  const yScale = 1.6 + parameters.values[0] * 0.55;
  return {
    x: cosine / denominator,
    y: yScale * sine * cosine / denominator,
  };
}

function buildHypotrochoidPoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const rotationalOrder = integerFromUnit(parameters.values[0], 3, 7);
  const orbitRadius = rotationalOrder - 1;
  const penDistance = 0.75
    + parameters.detailScale * (0.55 + parameters.values[1] * 0.75);
  return {
    x: orbitRadius * Math.cos(t)
      + penDistance * Math.cos(orbitRadius * t),
    y: orbitRadius * Math.sin(t)
      - penDistance * Math.sin(orbitRadius * t),
  };
}

function buildSuperformulaPoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const rotationalOrder = integerFromUnit(parameters.values[0], 3, 8);
  const angularTerm = rotationalOrder * t / 4;
  const curvePower = 1.7 + parameters.detailScale * 2.4;
  const radialPower = 0.72 + parameters.values[1] * 0.82;
  const radius = (
    Math.abs(Math.cos(angularTerm)) ** curvePower
      + Math.abs(Math.sin(angularTerm)) ** curvePower
  ) ** (-1 / radialPower);
  return polarPoint(t, radius);
}

function buildPolarWeavePoint(
  progress: number,
  parameters: CurveParameters,
): CurvePoint {
  const t = progress * TAU;
  const rotationalOrder = integerFromUnit(parameters.values[0], 3, 8);
  const primaryAmplitude = (0.18 + parameters.values[1] * 0.14)
    * parameters.detailScale;
  const secondaryAmplitude = 0.05 + parameters.values[2] * 0.09;
  const radius = 1
    + primaryAmplitude * Math.cos(rotationalOrder * t)
    + secondaryAmplitude * Math.cos(rotationalOrder * 2 * t);
  return polarPoint(t, radius);
}

const CURVE_POINT_BUILDERS: readonly CurvePointBuilder[] = [
  buildRadialPetalPoint,
  buildHarmonicOrbitPoint,
  buildRosePoint,
  buildLissajousPoint,
  buildLemniscatePoint,
  buildHypotrochoidPoint,
  buildSuperformulaPoint,
  buildPolarWeavePoint,
];

function rotatePoint(point: CurvePoint, angle: number): CurvePoint {
  const cosine = Math.cos(angle);
  const sine = Math.sin(angle);
  return {
    x: point.x * cosine - point.y * sine,
    y: point.x * sine + point.y * cosine,
  };
}

/** 中文注释：只围绕数学原点等比缩放，不按包围盒平移，保证圆心始终映射到 50,50。 */
function fitCurveAtCenter(points: readonly CurvePoint[]): CurvePoint[] {
  const maximumExtent = points.reduce(
    (result, point) => Math.max(result, Math.abs(point.x), Math.abs(point.y)),
    0,
  );
  const scale = CURVE_TARGET_RADIUS / Math.max(maximumExtent, 1);
  return points.map((point) => ({
    x: 50 + point.x * scale,
    y: 50 + point.y * scale,
  }));
}

function buildCurvePath(seed: string): string {
  const random = createSeededRandom(`curve:${seed}`);
  const buildPoint = CURVE_POINT_BUILDERS[
    randomInteger(random, 0, CURVE_POINT_BUILDERS.length - 1)
  ];
  const parameters: CurveParameters = {
    detailScale: 0.56 + random() * 0.4,
    values: Array.from({ length: 4 }, () => random()),
  };
  const rotation = random() * TAU;
  const points = Array.from({ length: CURVE_STEP_COUNT + 1 }, (_, index) => {
    const progress = index / CURVE_STEP_COUNT;
    return rotatePoint(buildPoint(progress, parameters), rotation);
  });
  return fitCurveAtCenter(points)
    .map((point, index) => (
      `${index === 0 ? "M" : "L"}${point.x.toFixed(2)} ${point.y.toFixed(2)}`
    ))
    .join(" ");
}

export function getSeededAvatarAppearance(
  seed: string,
): SeededAvatarAppearance {
  const normalizedSeed = normalizeAvatarSeed(seed);
  const palette = SEEDED_AVATAR_PALETTES[
    hashSeed(`palette:${normalizedSeed}`) % SEEDED_AVATAR_PALETTES.length
  ];
  return {
    backgroundColor: palette.backgroundColor,
    foregroundColor: palette.foregroundColor,
    pathData: buildCurvePath(normalizedSeed),
  };
}

/** 为只能接收图片地址的消息头像生成同源静态资源。 */
export function getSeededAvatarDataUrl(seed: string): string {
  const normalizedSeed = normalizeAvatarSeed(seed);
  const cachedDataUrl = seededAvatarDataUrlCache.get(normalizedSeed);
  if (cachedDataUrl) {
    return cachedDataUrl;
  }
  const appearance = getSeededAvatarAppearance(normalizedSeed);
  const svg = [
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">',
    `<rect width="100" height="100" fill="${appearance.backgroundColor}"/>`,
    `<path d="${appearance.pathData}" fill="none" stroke="${appearance.foregroundColor}" stroke-linecap="round" stroke-linejoin="round" stroke-width="3.75"/>`,
    "</svg>",
  ].join("");
  const dataUrl = `data:image/svg+xml,${encodeURIComponent(svg)}`;
  if (seededAvatarDataUrlCache.size >= SEEDED_AVATAR_DATA_URL_CACHE_LIMIT) {
    const oldestSeed = seededAvatarDataUrlCache.keys().next().value;
    if (oldestSeed) {
      seededAvatarDataUrlCache.delete(oldestSeed);
    }
  }
  seededAvatarDataUrlCache.set(normalizedSeed, dataUrl);
  return dataUrl;
}
