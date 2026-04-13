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

function getDefaultRadius(variant: "toolbar" | "panel" | "focus" | "dialog" | "chip" | "switch"): number {
  if (variant === "panel") {
    return 28;
  }
  if (variant === "dialog") {
    return 34;
  }
  return 999;
}

function BaseGlassPrimitive<T extends GlassPrimitiveTagName>({
  radius,
  true_glass = false,
  ...props
}: GlassPrimitiveProps<T> & { variant: "toolbar" | "panel" | "focus" | "dialog" | "chip" | "switch" }) {
  return (
    <LiquidGlassPanel
      {...props}
      enable_true_glass={true_glass}
      radius={radius ?? getDefaultRadius(props.variant)}
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
 * 中文注释：GlassFocusControl 只保留给少数高关注控件，
 * 当前用于 launcher Hero 的发送按钮。
 */
export function GlassFocusControl<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="focus" />;
}

/**
 * 中文注释：GlassDialog 用于模态对话框容器，
 * 高模糊 + 低折射，宽 bezel 配合大圆角。
 */
export function GlassDialog<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="dialog" />;
}

/**
 * 中文注释：GlassChip 用于小面积交互元素（标签、按钮），
 * 低模糊微折射，保持文字清晰。
 */
export function GlassChip<T extends GlassPrimitiveTagName = "div">(
  props: GlassPrimitiveProps<T>,
) {
  return <BaseGlassPrimitive {...props} variant="chip" />;
}
