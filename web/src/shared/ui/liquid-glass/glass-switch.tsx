/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：glass-switch.tsx
# @Date   ：2026-04-12 21:28
# @Author ：leemysw
# 2026-04-12 21:28   Create
# =====================================================
*/

"use client";

import { useEffect, useId, useRef, useState } from "react";

import { cn } from "@/lib/utils";

import { supports_true_liquid_glass } from "./liquid-glass-engine";

interface GlassSwitchProps {
  checked: boolean;
  disabled?: boolean;
  on_change: (checked: boolean) => void;
  class_name?: string;
}

const SOURCE_TRACK_WIDTH = 160;
const SOURCE_TRACK_HEIGHT = 67;
const SOURCE_THUMB_WIDTH = 146;
const SOURCE_THUMB_HEIGHT = 92;
const SOURCE_THUMB_RADIUS = 46;
const SOURCE_THUMB_OFFSET_X = -21.95;
const SOURCE_STATIC_THUMB_TRAVEL_X = 57.9;
const SOURCE_STATIC_THUMB_SCALE = 0.65;
const SOURCE_THUMB_TRAVEL_X = 40.9;
const SOURCE_THUMB_SCALE = 0.9;
const SOURCE_FILTER_BLUR = 0.2;
const SOURCE_FILTER_SATURATION = "6";
const SOURCE_FILTER_SPECULAR_FADE = 0.5;
const SOURCE_FILTER_DISPLACEMENT_SCALE = 22.26064761799501;
const LOCAL_DISPLACEMENT_MAP_URL = "/liquid-glass/displacement-map.png";
const LOCAL_SPECULAR_MAP_URL = "/liquid-glass/specular-map.png";
const TARGET_TRACK_HEIGHT = 28;
const SCALE_RATIO = TARGET_TRACK_HEIGHT / SOURCE_TRACK_HEIGHT;
const TRACK_WIDTH = Math.round(SOURCE_TRACK_WIDTH * SCALE_RATIO);
const TRACK_HEIGHT = TARGET_TRACK_HEIGHT;
const THUMB_WIDTH = SOURCE_THUMB_WIDTH * SCALE_RATIO;
const THUMB_HEIGHT = SOURCE_THUMB_HEIGHT * SCALE_RATIO;
const THUMB_RADIUS = SOURCE_THUMB_RADIUS * SCALE_RATIO;
const THUMB_OFFSET_X = SOURCE_THUMB_OFFSET_X * SCALE_RATIO;
const STATIC_THUMB_TRAVEL_X = SOURCE_STATIC_THUMB_TRAVEL_X * SCALE_RATIO;
const THUMB_TRAVEL_X = SOURCE_THUMB_TRAVEL_X * SCALE_RATIO;

/**
 * 中文注释：共享 glass 开关采用 switch 专用折射滤镜，
 * 不再复用通用 panel 材质，避免 thumb 的曲面和 specular 被抽象层抹平。
 */
export function GlassSwitch({
  checked,
  disabled = false,
  on_change,
  class_name,
}: GlassSwitchProps) {
  const raw_filter_id = useId();
  const filter_id = `glass-switch-thumb-${raw_filter_id.replace(/:/g, "")}`;
  const [can_use_true_glass, set_can_use_true_glass] = useState(false);
  const [is_pressed, set_is_pressed] = useState(false);
  const [is_transitioning, set_is_transitioning] = useState(false);
  const previous_checked_ref = useRef(checked);

  useEffect(() => {
    set_can_use_true_glass(supports_true_liquid_glass());
  }, []);

  useEffect(() => {
    /**
     * 中文注释：首屏渲染时 previous_checked_ref 与当前值一致，
     * 不应把初始化当成一次开关动画，否则会错误显示 glass 覆盖层。
     */
    if (previous_checked_ref.current === checked) {
      return;
    }

    previous_checked_ref.current = checked;
    set_is_transitioning(true);
  }, [checked]);

  useEffect(() => {
    if (!disabled) {
      return;
    }

    set_is_pressed(false);
    set_is_transitioning(false);
  }, [disabled]);

  const show_interaction_filter = can_use_true_glass
    && (is_pressed || is_transitioning);

  return (
    <button
      aria-checked={checked}
      className={cn(
        "relative inline-flex shrink-0 items-center overflow-visible rounded-full transition-[background-color] duration-(--motion-duration-fast) ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(88,196,94,0.32)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent",
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
        class_name,
      )}
      onClick={() => {
        if (!disabled) {
          on_change(!checked);
        }
      }}
      onBlur={() => {
        set_is_pressed(false);
      }}
      onKeyDown={(event) => {
        if (disabled) {
          return;
        }

        if (event.key === " " || event.key === "Enter") {
          set_is_pressed(true);
        }
      }}
      onKeyUp={(event) => {
        if (event.key === " " || event.key === "Enter") {
          set_is_pressed(false);
        }
      }}
      onPointerCancel={() => {
        set_is_pressed(false);
      }}
      onPointerDown={(event) => {
        if (disabled) {
          return;
        }

        event.currentTarget.setPointerCapture(event.pointerId);
        set_is_pressed(true);
      }}
      onPointerUp={(event) => {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
          event.currentTarget.releasePointerCapture(event.pointerId);
        }
        set_is_pressed(false);
      }}
      role="switch"
      type="button"
      style={{
        width: `${TRACK_WIDTH}px`,
        height: `${TRACK_HEIGHT}px`,
        backgroundColor: checked ? "rgba(59,191,78,0.93333)" : "rgba(198,201,210,0.82)",
      }}
    >
      {can_use_true_glass ? (
        <svg
          aria-hidden="true"
          className="pointer-events-none absolute h-0 w-0 overflow-hidden"
          colorInterpolationFilters="sRGB"
          focusable="false"
        >
          <defs>
            <filter id={filter_id}>
              <feGaussianBlur
                in="SourceGraphic"
                result="blurred_source"
                stdDeviation={SOURCE_FILTER_BLUR}
              />
              <feImage
                href={LOCAL_DISPLACEMENT_MAP_URL}
                result="displacement_map"
                x={0}
                y={0}
                width={THUMB_WIDTH}
                height={THUMB_HEIGHT}
              />
              <feDisplacementMap
                in="blurred_source"
                in2="displacement_map"
                result="displaced"
                scale={SOURCE_FILTER_DISPLACEMENT_SCALE}
                xChannelSelector="R"
                yChannelSelector="G"
              />
              <feColorMatrix
                in="displaced"
                result="displaced_saturated"
                type="saturate"
                values={SOURCE_FILTER_SATURATION}
              />
              <feImage
                href={LOCAL_SPECULAR_MAP_URL}
                result="specular_layer"
                x={0}
                y={0}
                width={THUMB_WIDTH}
                height={THUMB_HEIGHT}
              />
              <feComposite
                in="displaced_saturated"
                in2="specular_layer"
                operator="in"
                result="specular_saturated"
              />
              <feComponentTransfer
                in="specular_layer"
                result="specular_faded"
              >
                <feFuncA type="linear" slope={SOURCE_FILTER_SPECULAR_FADE} />
              </feComponentTransfer>
              <feBlend
                in="specular_saturated"
                in2="displaced"
                mode="normal"
                result="with_saturation"
              />
              <feBlend
                in="specular_faded"
                in2="with_saturation"
                mode="normal"
              />
            </filter>
          </defs>
        </svg>
      ) : null}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute rounded-full transition-[transform,opacity] duration-(--motion-duration-fast) ease-out will-change-transform"
        style={{
          width: `${THUMB_WIDTH}px`,
          height: `${THUMB_HEIGHT}px`,
          marginLeft: `${THUMB_OFFSET_X}px`,
          top: `${TRACK_HEIGHT / 2}px`,
          borderRadius: `${THUMB_RADIUS}px`,
          backgroundColor: "rgb(255, 255, 255)",
          boxShadow: "0 4px 22px rgba(0, 0, 0, 0.1)",
          opacity: show_interaction_filter ? 0 : 1,
          transform: `translateX(${checked ? STATIC_THUMB_TRAVEL_X : 0}px) translateY(-50%) scale(${SOURCE_STATIC_THUMB_SCALE})`,
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute rounded-full transition-[transform,opacity] duration-(--motion-duration-fast) ease-out will-change-transform"
        onTransitionEnd={(event) => {
          if (event.propertyName === "transform") {
            set_is_transitioning(false);
          }
        }}
        style={{
          width: `${THUMB_WIDTH}px`,
          height: `${THUMB_HEIGHT}px`,
          marginLeft: `${THUMB_OFFSET_X}px`,
          top: `${TRACK_HEIGHT / 2}px`,
          borderRadius: `${THUMB_RADIUS}px`,
          backgroundColor: "rgba(255, 255, 255, 0.098)",
          boxShadow:
            "0 4px 22px rgba(0, 0, 0, 0.1), 2px 7px 24px rgba(0, 0, 0, 0.09) inset, -2px -7px 24px rgba(255, 255, 255, 0.09) inset",
          backdropFilter: show_interaction_filter ? `url(#${filter_id})` : undefined,
          WebkitBackdropFilter: show_interaction_filter ? `url(#${filter_id})` : undefined,
          opacity: show_interaction_filter ? 1 : 0,
          transform: `translateX(${checked ? THUMB_TRAVEL_X : 0}px) translateY(-50%) scale(${SOURCE_THUMB_SCALE})`,
        }}
      />
    </button>
  );
}
