/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：liquid-glass-presets.ts
# @Date   ：2026-04-11 23:41
# @Author ：leemysw
# 2026-04-11 23:41   Create
# =====================================================
*/

export type LiquidGlassVariant = "toolbar" | "panel" | "focus" | "dialog" | "chip" | "switch" | "magnifier";

export interface LiquidGlassVariantPreset {
  bezel: number;
  blur: number;
  distortion: number;
  saturation: number;
  surface_profile?: "convex" | "lip";
  background: string;
  border_color: string;
  highlight_background: string;
  highlight_opacity: number;
  shadow: string;
  specular_power?: number;
  specular_opacity?: number;
}

/**
 * 中文注释：Liquid glass 只保留少数语义形态，
 * 避免页面层继续按视觉细节随意拼装 preset。
 */
export const LIQUID_GLASS_VARIANTS: Record<LiquidGlassVariant, LiquidGlassVariantPreset> = {
  toolbar: {
    bezel: 10,
    blur: 14,
    distortion: 12,
    saturation: 132,
    surface_profile: "convex",
    background: "color-mix(in srgb, var(--surface-panel-background) 72%, transparent)",
    border_color: "color-mix(in srgb, var(--surface-panel-border) 78%, transparent)",
    highlight_background: "radial-gradient(74% 54% at 14% 4%, rgba(255,255,255,0.18), rgba(255,255,255,0) 56%)",
    highlight_opacity: 0.72,
    shadow: "0 12px 32px rgba(15, 23, 42, 0.08)",
  },
  panel: {
    bezel: 14,
    blur: 18,
    distortion: 16,
    saturation: 142,
    surface_profile: "convex",
    background: "var(--surface-panel-background)",
    border_color: "var(--surface-panel-border)",
    highlight_background: "radial-gradient(72% 52% at 16% 6%, rgba(255,255,255,0.2), rgba(255,255,255,0) 58%), linear-gradient(180deg, rgba(255,255,255,0.12), rgba(255,255,255,0.02) 52%, rgba(255,255,255,0) 100%)",
    highlight_opacity: 0.88,
    shadow: "var(--surface-panel-shadow)",
  },
  focus: {
    bezel: 12,
    blur: 16,
    distortion: 14,
    saturation: 150,
    surface_profile: "convex",
    background: "color-mix(in srgb, var(--surface-popover-background) 90%, transparent)",
    border_color: "color-mix(in srgb, var(--surface-popover-border) 92%, transparent)",
    highlight_background: "radial-gradient(78% 56% at 18% 4%, rgba(255,255,255,0.24), rgba(255,255,255,0) 56%), linear-gradient(180deg, rgba(255,255,255,0.16), rgba(255,255,255,0.04) 48%, rgba(255,255,255,0) 100%)",
    highlight_opacity: 0.92,
    shadow: "var(--surface-popover-shadow)",
  },
  /**
   * 中文注释：Dialog 材质 — 大面积模态场景，高模糊强分隔，
   * 低折射避免文字干扰，宽 bezel 配合 radius-shell-xl (34px)。
   */
  dialog: {
    bezel: 18,
    blur: 22,
    distortion: 10,
    saturation: 155,
    surface_profile: "convex",
    background: "var(--modal-dialog-surface-background)",
    border_color: "var(--modal-dialog-surface-border)",
    highlight_background: "radial-gradient(68% 48% at 12% 4%, rgba(255,255,255,0.16), rgba(255,255,255,0) 52%), linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.02) 40%, rgba(255,255,255,0) 100%)",
    highlight_opacity: 0.82,
    shadow: "var(--modal-dialog-surface-shadow)",
    specular_power: 2.6,
    specular_opacity: 0.72,
  },
  /**
   * 中文注释：Chip 材质 — 小面积交互元素，低模糊保持清晰，
   * 微弱折射和 specular，用于标签、按钮等。
   */
  chip: {
    bezel: 6,
    blur: 8,
    distortion: 6,
    saturation: 128,
    surface_profile: "convex",
    background: "var(--chip-default-background)",
    border_color: "var(--chip-default-border)",
    highlight_background: "radial-gradient(64% 42% at 16% 6%, rgba(255,255,255,0.14), rgba(255,255,255,0) 48%)",
    highlight_opacity: 0.64,
    shadow: "var(--chip-default-shadow)",
    specular_power: 3.0,
    specular_opacity: 0.48,
  },
  /**
   * 提高折射与高光强度，让中部凹槽和边缘凸面更明显。
   */
  switch: {
    bezel: 18,
    blur: 0.2,
    distortion: 22.26064761799501,
    saturation: 600,
    surface_profile: "lip",
    background: "rgba(255,255,255,1)",
    border_color: "rgba(255,255,255,0)",
    highlight_background: "radial-gradient(82% 68% at 18% 4%, rgba(255,255,255,0.26), rgba(255,255,255,0) 60%), linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.02) 48%, rgba(255,255,255,0) 100%)",
    highlight_opacity: 0,
    shadow: "0 4px 22px rgba(0,0,0,0.10)",
    specular_power: 1.9,
    specular_opacity: 0.5,
  },
  /**
   * 中文注释：Magnifier 材质 — 放大镜效果，高折射产生真实放大感，
   * 强阴影多层叠加模拟深度，用于 logo 按钮等视觉焦点。
   */
  magnifier: {
    bezel: 24,
    blur: 4,
    distortion: 48,
    saturation: 900,
    surface_profile: "convex",
    background: "var(--surface-avatar-background)",
    border_color: "var(--surface-avatar-border)",
    highlight_background: "radial-gradient(86% 72% at 22% 8%, rgba(255,255,255,0.32), rgba(255,255,255,0) 64%), linear-gradient(180deg, rgba(255,255,255,0.18), rgba(255,255,255,0.04) 52%, rgba(255,255,255,0) 100%)",
    highlight_opacity: 0.96,
    shadow: "var(--surface-avatar-shadow)",
    specular_power: 2.8,
    specular_opacity: 0.64,
  },
};
