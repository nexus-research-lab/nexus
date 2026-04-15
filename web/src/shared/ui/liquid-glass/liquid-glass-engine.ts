/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：liquid-glass-engine.ts
# @Date   ：2026-04-11 11:34
# @Author ：leemysw
# 2026-04-11 11:34   Create
# =====================================================
*/

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

// 中文注释：squircle 曲面轮廓 y = ⁴√(1-(1-x)⁴)，
// 比 smootherstep 幂曲线产生更柔和的边缘过渡。
function squircle_surface_profile(x: number): number {
  const t = clamp(x, 0, 1);
  return Math.pow(1 - Math.pow(1 - t, 4), 0.25);
}

function lip_surface_profile(x: number): number {
  const t = clamp(x, 0, 1);
  const convex = squircle_surface_profile(1 - t);
  const concave = squircle_surface_profile(t);
  const blend = smootherstep(t);
  return convex * (1 - blend) - concave * blend * 0.28;
}

function get_rounded_rect_sdf(x: number, y: number, width: number, height: number, radius: number): number {
  const half_width = width / 2;
  const half_height = height / 2;
  const dx = Math.abs(x - half_width) - (half_width - radius);
  const dy = Math.abs(y - half_height) - (half_height - radius);
  const outer_x = Math.max(dx, 0);
  const outer_y = Math.max(dy, 0);
  return Math.hypot(outer_x, outer_y) + Math.min(Math.max(dx, dy), 0) - radius;
}

function get_sdf_normal(x: number, y: number, width: number, height: number, radius: number): Vector2 {
  const epsilon = 0.85;
  const dx = get_rounded_rect_sdf(x + epsilon, y, width, height, radius)
    - get_rounded_rect_sdf(x - epsilon, y, width, height, radius);
  const dy = get_rounded_rect_sdf(x, y + epsilon, width, height, radius)
    - get_rounded_rect_sdf(x, y - epsilon, width, height, radius);
  const length = Math.hypot(dx, dy);

  if (length < 0.0001) {
    return { x: 0, y: -1 };
  }

  return {
    x: dx / length,
    y: dy / length,
  };
}

function create_canvas_context(width: number, height: number): CanvasRenderingContext2D | null {
  if (typeof document === "undefined") {
    return null;
  }

  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  return canvas.getContext("2d");
}

function encode_vector_channel(value: number): number {
  return Math.round(clamp(128 + value * 127, 0, 255));
}

function build_cache_key({
  width,
  height,
  radius,
  bezel,
  surface_profile = "convex",
  light_angle_deg = DEFAULT_LIGHT_ANGLE_DEG,
  specular_power = 2.2,
  specular_opacity = 1.0,
}: LiquidGlassAssetOptions): string {
  // 中文注释：折射贴图按尺寸档位复用，避免每个像素级尺寸都生成新位移图。
  return [
    quantize(width, CACHE_SIZE_STEP),
    quantize(height, CACHE_SIZE_STEP),
    quantize(radius, CACHE_RADIUS_STEP),
    quantize(bezel, CACHE_RADIUS_STEP),
    surface_profile,
    Math.round(light_angle_deg * 10),
    Math.round(specular_power * 10),
    Math.round(specular_opacity * 100),
  ].join(":");
}

function create_glass_assets({
  width,
  height,
  radius,
  bezel,
  surface_profile = "convex",
  light_angle_deg = DEFAULT_LIGHT_ANGLE_DEG,
  specular_power = 2.2,
  specular_opacity = 1.0,
}: LiquidGlassAssetOptions): LiquidGlassAssetBundle | null {
  const scale_ratio = clamp(MAX_SAMPLE_EDGE / Math.max(width, height), MIN_SAMPLE_SIZE / Math.min(width, height), 1);
  const sample_width = Math.max(MIN_SAMPLE_SIZE, Math.round(width * scale_ratio));
  const sample_height = Math.max(MIN_SAMPLE_SIZE, Math.round(height * scale_ratio));
  const sample_radius = clamp(radius * scale_ratio, 4, Math.min(sample_width, sample_height) / 2);
  const sample_bezel = clamp(bezel * scale_ratio, 6, Math.min(sample_radius, sample_width / 3, sample_height / 3));
  const displacement_context = create_canvas_context(sample_width, sample_height);
  const highlight_context = create_canvas_context(sample_width, sample_height);

  if (!displacement_context || !highlight_context) {
    return null;
  }

  const displacement_data = displacement_context.createImageData(sample_width, sample_height);
  const highlight_data = highlight_context.createImageData(sample_width, sample_height);
  const displacement_buffer = displacement_data.data;
  const highlight_buffer = highlight_data.data;
  const light_radians = light_angle_deg * (Math.PI / 180);
  const light_direction = {
    x: Math.cos(light_radians),
    y: Math.sin(light_radians),
  };

  // 中文注释：这里按“圆角矩形 SDF + 法线近似”生成折射位移图，
  // 不是简单高斯模糊叠层，而是真正给 feDisplacementMap 提供向量场。
  for (let y = 0; y < sample_height; y += 1) {
    for (let x = 0; x < sample_width; x += 1) {
      const pixel_index = (y * sample_width + x) * 4;
      const signed_distance = get_rounded_rect_sdf(x + 0.5, y + 0.5, sample_width, sample_height, sample_radius);

      displacement_buffer[pixel_index] = 128;
      displacement_buffer[pixel_index + 1] = 128;
      displacement_buffer[pixel_index + 2] = 128;
      displacement_buffer[pixel_index + 3] = 255;
      highlight_buffer[pixel_index] = 255;
      highlight_buffer[pixel_index + 1] = 255;
      highlight_buffer[pixel_index + 2] = 255;
      highlight_buffer[pixel_index + 3] = 0;

      if (signed_distance > 0) {
        continue;
      }

      const distance_from_edge = -signed_distance;
      if (distance_from_edge > sample_bezel * 1.18) {
        continue;
      }

      const outward_normal = get_sdf_normal(x + 0.5, y + 0.5, sample_width, sample_height, sample_radius);
      const inward_normal = {
        x: -outward_normal.x,
        y: -outward_normal.y,
      };
      const normalized_bezel_position = clamp(distance_from_edge / sample_bezel, 0, 1);
      // 中文注释：switch 走 lip 轮廓，外缘向内折射，内槽轻微向外散，
      // 让中部看起来被“拉远”，更接近参考站的玻璃开关。
      const profile_strength = surface_profile === "lip"
        ? lip_surface_profile(normalized_bezel_position)
        : squircle_surface_profile(1 - normalized_bezel_position);
      const displacement_strength = profile_strength * (0.82 + (1 - normalized_bezel_position) * 0.18);

      displacement_buffer[pixel_index] = encode_vector_channel(inward_normal.x * displacement_strength);
      displacement_buffer[pixel_index + 1] = encode_vector_channel(inward_normal.y * displacement_strength);

      const light_facing = Math.max(0, outward_normal.x * light_direction.x + outward_normal.y * light_direction.y);
      const rim_strength = Math.pow(1 - normalized_bezel_position, 2.35);
      const diffuse_glow = Math.pow(1 - normalized_bezel_position, 3.8) * 0.18;
      const highlight_alpha = clamp((Math.pow(light_facing, specular_power) * rim_strength + diffuse_glow) * specular_opacity * 255, 0, 255);
      highlight_buffer[pixel_index + 3] = Math.round(highlight_alpha);
    }
  }

  displacement_context.putImageData(displacement_data, 0, 0);
  highlight_context.putImageData(highlight_data, 0, 0);

  return {
    displacement_map_url: displacement_context.canvas.toDataURL("image/png"),
    highlight_map_url: highlight_context.canvas.toDataURL("image/png"),
  };
}

export function get_liquid_glass_assets(options: LiquidGlassAssetOptions): LiquidGlassAssetBundle | null {
  const cache_key = build_cache_key(options);
  const cached = LIQUID_GLASS_CACHE.get(cache_key);
  if (cached) {
    return cached;
  }

  const assets = create_glass_assets(options);
  if (!assets) {
    return null;
  }

  LIQUID_GLASS_CACHE.set(cache_key, assets);
  return assets;
}

export function supports_true_liquid_glass(): boolean {
  if (typeof window === "undefined" || typeof CSS === "undefined" || typeof navigator === "undefined") {
    return false;
  }

  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return false;
  }

  const supports_backdrop = CSS.supports("backdrop-filter", "blur(1px)")
    || CSS.supports("-webkit-backdrop-filter", "blur(1px)");
  if (!supports_backdrop) {
    return false;
  }

  const user_agent = navigator.userAgent;
  const is_firefox = /Firefox\//i.test(user_agent);
  const navigator_connection = navigator as Navigator & {
    connection?: {
      saveData?: boolean;
    };
  };
  if (navigator_connection.connection?.saveData) {
    return false;
  }

  /**
   * 这里不再用浏览器品牌做硬编码拦截。
   * 我们只排除已知表现不稳定的 Firefox，其余浏览器交给能力检测和实际渲染结果决定。
   */
  return !is_firefox;
}
