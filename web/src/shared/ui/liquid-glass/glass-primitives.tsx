/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：glass-primitives.tsx
# @Date   ：2026-04-12 17:08
# @Author ：leemysw
# 2026-04-12 17:08   Create
# =====================================================
*/

"use client";

import { LiquidGlassPanel, type LiquidGlassPanelProps } from "./liquid-glass-panel";

type GlassPrimitiveTagName = "aside" | "button" | "div" | "main" | "section";

type GlassPrimitiveProps<T extends GlassPrimitiveTagName> =
  Omit<LiquidGlassPanelProps<T>, "enable_true_glass" | "variant"> & {
    true_glass?: boolean;
  };

function get_default_radius(variant: "toolbar" | "panel" | "focus" | "dialog" | "chip" | "switch" | "magnifier"): number {
  if (variant === "panel") {
    return 28;
  }
  if (variant === "dialog") {
    return 34;
  }
  if (variant === "magnifier") {
    return 999;
  }
  return 999;
}

function BaseGlassPrimitive<T extends GlassPrimitiveTagName>({
  radius,
  true_glass = false,
  ...props
}: GlassPrimitiveProps<T> & { variant: "toolbar" | "panel" | "focus" | "dialog" | "chip" | "switch" | "magnifier" }) {
  return (
    <LiquidGlassPanel
      {...props}
      enable_true_glass={true_glass}
      radius={radius ?? get_default_radius(props.variant)}
      variant={props.variant}
    />
  );
}

/**
 * 中文注释：工具条面只服务于小范围、横向、轻量交互，
 * 比如 launcher 的入口胶囊或系统浮层里的轻操作条。
 */
export function GlassToolbar<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="toolbar" />;
}

/**
 * 中文注释：GlassPanel 是大面承载层，只给 Hero 主面和系统级浮层使用。
 */
export function GlassPanel<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="panel" />;
}

/**
 * 中文注释：GlassMagnifier 用于放大镜效果，高折射产生真实放大感，
 * 强阴影多层叠加模拟深度，用于 logo 按钮等视觉焦点。
 */
export function GlassMagnifier<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="magnifier" />;
}
