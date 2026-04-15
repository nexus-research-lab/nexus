/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：liquid-glass-panel.tsx
# @Date   ：2026-04-11 11:34
# @Author ：leemysw
# 2026-04-11 11:34   Create
# =====================================================
*/

"use client";

import { ComponentPropsWithoutRef, CSSProperties, ReactNode, useEffect, useId, useMemo, useRef, useState } from "react";

import { cn } from "@/lib/utils";

import { get_liquid_glass_assets, supports_true_liquid_glass } from "./liquid-glass-engine";
import { LIQUID_GLASS_VARIANTS, LiquidGlassVariant } from "./liquid-glass-presets";

type LiquidGlassTagName = "aside" | "button" | "div" | "main" | "section";
type LiquidGlassContentLayout = "natural" | "fill" | "fill-flex";

interface LiquidGlassPanelOwnProps {
  children: ReactNode;
  class_name?: string;
  content_class_name?: string;
  content_layout?: LiquidGlassContentLayout;
  enable_true_glass?: boolean;
  surface_style?: CSSProperties;
  style?: CSSProperties;
  radius?: number;
  variant?: LiquidGlassVariant;
}

type LiquidGlassNativeProps<T extends LiquidGlassTagName> =
  Omit<ComponentPropsWithoutRef<T>, "children" | "className" | "style">;

export type LiquidGlassPanelProps<T extends LiquidGlassTagName = "div"> =
  LiquidGlassPanelOwnProps & {
    tag_name?: T;
  } & LiquidGlassNativeProps<T>;

function useMeasuredElementSize() {
  const [element, set_element] = useState<HTMLElement | null>(null);
  const [size, set_size] = useState({ width: 0, height: 0 });

  useEffect(() => {
    if (!element) {
      return;
    }

    const observer = new ResizeObserver(([entry]) => {
      const next_width = Math.round(entry.contentRect.width);
      const next_height = Math.round(entry.contentRect.height);
      set_size((current_size) => (
        current_size.width === next_width && current_size.height === next_height
          ? current_size
          : { width: next_width, height: next_height }
      ));
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);

  return { set_element, size };
}

export function LiquidGlassPanel<T extends LiquidGlassTagName = "div">({
  children,
  class_name,
  content_class_name,
  content_layout = "natural",
  enable_true_glass = false,
  surface_style,
  style,
  radius = 32,
  tag_name = "div" as T,
  variant = "panel",
  ...native_props
}: LiquidGlassPanelProps<T>) {
  const TagName = tag_name as any;
  const { set_element, size } = useMeasuredElementSize();
  const raw_filter_id = useId();
  const filter_id = `liquid-glass-${raw_filter_id.replace(/:/g, "")}`;
  const [can_use_true_glass, set_can_use_true_glass] = useState(false);
  const variant_preset = LIQUID_GLASS_VARIANTS[variant];
  const displacement_ref = useRef<SVGFEDisplacementMapElement | null>(null);
  const raf_id_ref = useRef(0);

  useEffect(() => {
    // 中文注释：默认全部退回静态材质，只有显式声明的原语才允许尝试真折射。
    set_can_use_true_glass(enable_true_glass && supports_true_liquid_glass());
  }, [enable_true_glass]);

  const assets = useMemo(() => {
    if (!can_use_true_glass || size.width < 24 || size.height < 24) {
      return null;
    }

    return get_liquid_glass_assets({
      width: size.width,
      height: size.height,
      radius,
      bezel: Math.max(8, Math.round(radius * 0.38), variant_preset.bezel),
      surface_profile: variant_preset.surface_profile ?? "convex",
      specular_power: variant_preset.specular_power,
      specular_opacity: variant_preset.specular_opacity,
    });
  }, [can_use_true_glass, radius, size.height, size.width, variant_preset.bezel, variant_preset.specular_power, variant_preset.specular_opacity, variant_preset.surface_profile]);

  const backdrop_filter = useMemo(() => {
    if (!can_use_true_glass || !assets) {
      return undefined;
    }

    return `url(#${filter_id}) blur(${variant_preset.blur}px) saturate(${variant_preset.saturation}%)`;
  }, [assets, can_use_true_glass, filter_id, variant_preset.blur, variant_preset.saturation]);

  // 中文注释：玻璃折射入场动画，scale 从 0 渐变到目标值，约 220ms，避免视觉突变。
  const target_distortion = variant_preset.distortion;

  useEffect(() => {
    const node = displacement_ref.current;
    if (!can_use_true_glass || !assets || !node) {
      return;
    }

    const duration = 220;
    const start = performance.now();
    node.scale.baseVal = 0;

    const animate = (now: number) => {
      const elapsed = now - start;
      const progress = Math.min(elapsed / duration, 1);
      // 中文注释：ease-out 缓动让折射效果自然展开
      const eased = 1 - Math.pow(1 - progress, 3);
      node.scale.baseVal = eased * target_distortion;

      if (progress < 1) {
        raf_id_ref.current = requestAnimationFrame(animate);
      }
    };

    raf_id_ref.current = requestAnimationFrame(animate);

    return () => {
      cancelAnimationFrame(raf_id_ref.current);
    };
  }, [can_use_true_glass, assets, target_distortion]);

  const surface_shape_style = useMemo<CSSProperties>(() => ({
    borderRadius: `${radius}px`,
  }), [radius]);

  const root_style = useMemo<CSSProperties>(() => ({
    contain: "paint",
    // 中文注释：把玻璃材质直接收回根节点，避免额外的 surface/content 包装层。
    ...surface_shape_style,
    background: variant_preset.background,
    boxShadow: `inset 0 0 0 1px ${variant_preset.border_color}, ${variant_preset.shadow}`,
    backdropFilter: backdrop_filter,
    WebkitBackdropFilter: backdrop_filter,
    transform: "translateZ(0)",
    ["--liquid-glass-highlight-background" as string]: variant_preset.highlight_background,
    ["--liquid-glass-highlight-opacity" as string]: String(variant_preset.highlight_opacity),
    ...surface_style,
    ...style,
  }), [
    backdrop_filter,
    style,
    surface_shape_style,
    surface_style,
    variant_preset.background,
    variant_preset.border_color,
    variant_preset.highlight_background,
    variant_preset.highlight_opacity,
    variant_preset.shadow,
  ]);

  const content_layout_class_name = useMemo(() => {
    if (content_layout === "fill") {
      return "h-full w-full min-h-0 min-w-0";
    }

    if (content_layout === "fill-flex") {
      return "flex h-full w-full min-h-0 min-w-0 flex-col";
    }

    return "";
  }, [content_layout]);

  return (
    <TagName
      {...(native_props as Record<string, unknown>)}
      className={cn(
        "liquid-glass-surface relative overflow-hidden",
        content_layout_class_name,
        content_class_name,
        class_name,
      )}
      ref={(node: HTMLElement | null) => set_element(node)}
      style={root_style}
    >
      {can_use_true_glass && assets ? (
        <svg
          aria-hidden="true"
          className="pointer-events-none absolute h-0 w-0 overflow-hidden"
          focusable="false"
        >
          <defs>
            <filter
              colorInterpolationFilters="sRGB"
              filterUnits="userSpaceOnUse"
              id={filter_id}
              primitiveUnits="userSpaceOnUse"
              x={0}
              y={0}
              width={size.width}
              height={size.height}
            >
              <feImage
                href={assets.displacement_map_url}
                x={0}
                y={0}
                width={size.width}
                height={size.height}
                result="liquid-glass-map"
              />
              <feDisplacementMap
                ref={displacement_ref}
                in="SourceGraphic"
                in2="liquid-glass-map"
                result="liquid-glass-refracted"
                scale={0}
                xChannelSelector="R"
                yChannelSelector="G"
              />
              <feImage
                href={assets.highlight_map_url}
                x={0}
                y={0}
                width={size.width}
                height={size.height}
                result="liquid-glass-highlight"
              />
              <feBlend
                in="liquid-glass-refracted"
                in2="liquid-glass-highlight"
                mode="screen"
              />
            </filter>
          </defs>
        </svg>
      ) : null}
      {children}
    </TagName>
  );
}
