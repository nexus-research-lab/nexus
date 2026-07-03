interface LiquidGlassAssetBundle {
  displacement_map_url: string;
  highlight_map_url: string;
}

interface LiquidGlassAssetOptions {
  width: number;
  height: number;
  radius: number;
  bezel: number;
  surface_profile?: "convex" | "lip";
  light_angle_deg?: number;
  specular_power?: number;
  specular_opacity?: number;
}

interface Vector2 {
  x: number;
  y: number;
}

const LIQUID_GLASS_CACHE = new Map<string, LiquidGlassAssetBundle>();
const DEFAULT_LIGHT_ANGLE_DEG = -48;
const MIN_SAMPLE_SIZE = 52;
const MAX_SAMPLE_EDGE = 260;
const CACHE_SIZE_STEP = 12;
const CACHE_RADIUS_STEP = 2;

function quantize(value: number, step: number): number {
  return Math.max(step, Math.round(value / step) * step);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function smootherstep(value: number): number {
  const normalized = clamp(value, 0, 1);
  return normalized * normalized * normalized * (normalized * (normalized * 6 - 15) + 10);
}

// squircle 曲面轮廓 y = ⁴√(1-(1-x)⁴)，
// 比 smootherstep 幂曲线产生更柔和的边缘过渡。
function squircleSurfaceProfile(x: number): number {
  const t = clamp(x, 0, 1);
  return Math.pow(1 - Math.pow(1 - t, 4), 0.25);
}

function lipSurfaceProfile(x: number): number {
  const t = clamp(x, 0, 1);
  const convex = squircleSurfaceProfile(1 - t);
  const concave = squircleSurfaceProfile(t);
  const blend = smootherstep(t);
  return convex * (1 - blend) - concave * blend * 0.28;
}

function getRoundedRectSdf(x: number, y: number, width: number, height: number, radius: number): number {
  const halfWidth = width / 2;
  const halfHeight = height / 2;
  const dx = Math.abs(x - halfWidth) - (halfWidth - radius);
  const dy = Math.abs(y - halfHeight) - (halfHeight - radius);
  const outerX = Math.max(dx, 0);
  const outerY = Math.max(dy, 0);
  return Math.hypot(outerX, outerY) + Math.min(Math.max(dx, dy), 0) - radius;
}

function getSdfNormal(x: number, y: number, width: number, height: number, radius: number): Vector2 {
  const epsilon = 0.85;
  const dx = getRoundedRectSdf(x + epsilon, y, width, height, radius)
    - getRoundedRectSdf(x - epsilon, y, width, height, radius);
  const dy = getRoundedRectSdf(x, y + epsilon, width, height, radius)
    - getRoundedRectSdf(x, y - epsilon, width, height, radius);
  const length = Math.hypot(dx, dy);

  if (length < 0.0001) {
    return { x: 0, y: -1 };
  }

  return {
    x: dx / length,
    y: dy / length,
  };
}

function createCanvasContext(width: number, height: number): CanvasRenderingContext2D | null {
  if (typeof document === "undefined") {
    return null;
  }

  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  return canvas.getContext("2d");
}

function encodeVectorChannel(value: number): number {
  return Math.round(clamp(128 + value * 127, 0, 255));
}

function buildCacheKey({
  width,
  height,
  radius,
  bezel,
  surface_profile: surfaceProfile = "convex",
  light_angle_deg: lightAngleDeg = DEFAULT_LIGHT_ANGLE_DEG,
  specular_power: specularPower = 2.2,
  specular_opacity: specularOpacity = 1.0,
}: LiquidGlassAssetOptions): string {
  // 折射贴图按尺寸档位复用，避免每个像素级尺寸都生成新位移图。
  return [
    quantize(width, CACHE_SIZE_STEP),
    quantize(height, CACHE_SIZE_STEP),
    quantize(radius, CACHE_RADIUS_STEP),
    quantize(bezel, CACHE_RADIUS_STEP),
    surfaceProfile,
    Math.round(lightAngleDeg * 10),
    Math.round(specularPower * 10),
    Math.round(specularOpacity * 100),
  ].join(":");
}

function createGlassAssets({
  width,
  height,
  radius,
  bezel,
  surface_profile: surfaceProfile = "convex",
  light_angle_deg: lightAngleDeg = DEFAULT_LIGHT_ANGLE_DEG,
  specular_power: specularPower = 2.2,
  specular_opacity: specularOpacity = 1.0,
}: LiquidGlassAssetOptions): LiquidGlassAssetBundle | null {
  const scaleRatio = clamp(MAX_SAMPLE_EDGE / Math.max(width, height), MIN_SAMPLE_SIZE / Math.min(width, height), 1);
  const sampleWidth = Math.max(MIN_SAMPLE_SIZE, Math.round(width * scaleRatio));
  const sampleHeight = Math.max(MIN_SAMPLE_SIZE, Math.round(height * scaleRatio));
  const sampleRadius = clamp(radius * scaleRatio, 4, Math.min(sampleWidth, sampleHeight) / 2);
  const sampleBezel = clamp(bezel * scaleRatio, 6, Math.min(sampleRadius, sampleWidth / 3, sampleHeight / 3));
  const displacementContext = createCanvasContext(sampleWidth, sampleHeight);
  const highlightContext = createCanvasContext(sampleWidth, sampleHeight);

  if (!displacementContext || !highlightContext) {
    return null;
  }

  const displacementData = displacementContext.createImageData(sampleWidth, sampleHeight);
  const highlightData = highlightContext.createImageData(sampleWidth, sampleHeight);
  const displacementBuffer = displacementData.data;
  const highlightBuffer = highlightData.data;
  const lightRadians = lightAngleDeg * (Math.PI / 180);
  const lightDirection = {
    x: Math.cos(lightRadians),
    y: Math.sin(lightRadians),
  };

  // 这里按“圆角矩形 SDF + 法线近似”生成折射位移图，
  // 不是简单高斯模糊叠层，而是真正给 feDisplacementMap 提供向量场。
  for (let y = 0; y < sampleHeight; y += 1) {
    for (let x = 0; x < sampleWidth; x += 1) {
      const pixelIndex = (y * sampleWidth + x) * 4;
      const signedDistance = getRoundedRectSdf(x + 0.5, y + 0.5, sampleWidth, sampleHeight, sampleRadius);

      displacementBuffer[pixelIndex] = 128;
      displacementBuffer[pixelIndex + 1] = 128;
      displacementBuffer[pixelIndex + 2] = 128;
      displacementBuffer[pixelIndex + 3] = 255;
      highlightBuffer[pixelIndex] = 255;
      highlightBuffer[pixelIndex + 1] = 255;
      highlightBuffer[pixelIndex + 2] = 255;
      highlightBuffer[pixelIndex + 3] = 0;

      if (signedDistance > 0) {
        continue;
      }

      const distanceFromEdge = -signedDistance;
      if (distanceFromEdge > sampleBezel * 1.18) {
        continue;
      }

      const outwardNormal = getSdfNormal(x + 0.5, y + 0.5, sampleWidth, sampleHeight, sampleRadius);
      const inwardNormal = {
        x: -outwardNormal.x,
        y: -outwardNormal.y,
      };
      const normalizedBezelPosition = clamp(distanceFromEdge / sampleBezel, 0, 1);
      // switch 走 lip 轮廓，外缘向内折射，内槽轻微向外散，
      // 让中部看起来被“拉远”，更接近参考站的玻璃开关。
      const profileStrength = surfaceProfile === "lip"
        ? lipSurfaceProfile(normalizedBezelPosition)
        : squircleSurfaceProfile(1 - normalizedBezelPosition);
      const displacementStrength = profileStrength * (0.82 + (1 - normalizedBezelPosition) * 0.18);

      displacementBuffer[pixelIndex] = encodeVectorChannel(inwardNormal.x * displacementStrength);
      displacementBuffer[pixelIndex + 1] = encodeVectorChannel(inwardNormal.y * displacementStrength);

      const lightFacing = Math.max(0, outwardNormal.x * lightDirection.x + outwardNormal.y * lightDirection.y);
      const rimStrength = Math.pow(1 - normalizedBezelPosition, 2.35);
      const diffuseGlow = Math.pow(1 - normalizedBezelPosition, 3.8) * 0.18;
      const highlightAlpha = clamp((Math.pow(lightFacing, specularPower) * rimStrength + diffuseGlow) * specularOpacity * 255, 0, 255);
      highlightBuffer[pixelIndex + 3] = Math.round(highlightAlpha);
    }
  }

  displacementContext.putImageData(displacementData, 0, 0);
  highlightContext.putImageData(highlightData, 0, 0);

  return {
    displacement_map_url: displacementContext.canvas.toDataURL("image/png"),
    highlight_map_url: highlightContext.canvas.toDataURL("image/png"),
  };
}

function getLiquidGlassAssets(options: LiquidGlassAssetOptions): LiquidGlassAssetBundle | null {
  const cacheKey = buildCacheKey(options);
  const cached = LIQUID_GLASS_CACHE.get(cacheKey);
  if (cached) {
    return cached;
  }

  const assets = createGlassAssets(options);
  if (!assets) {
    return null;
  }

  LIQUID_GLASS_CACHE.set(cacheKey, assets);
  return assets;
}

export function supportsTrueLiquidGlass(): boolean {
  if (typeof window === "undefined" || typeof CSS === "undefined" || typeof navigator === "undefined") {
    return false;
  }

  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return false;
  }

  const supportsBackdrop = CSS.supports("backdrop-filter", "blur(1px)")
    || CSS.supports("-webkit-backdrop-filter", "blur(1px)");
  if (!supportsBackdrop) {
    return false;
  }

  const userAgent = navigator.userAgent;
  const isFirefox = /Firefox\//i.test(userAgent);
  const navigatorConnection = navigator as Navigator & {
    connection?: {
      saveData?: boolean;
    };
  };
  if (navigatorConnection.connection?.saveData) {
    return false;
  }

  /**
   * 这里不再用浏览器品牌做硬编码拦截。
   * 我们只排除已知表现不稳定的 Firefox，其余浏览器交给能力检测和实际渲染结果决定。
   */
  return !isFirefox;
}
