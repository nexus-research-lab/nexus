"use client";

import { useEffect, useId, useRef, useState } from "react";

import { cn } from "@/lib/utils";

import { supportsTrueLiquidGlass } from "./liquid-glass-engine";

interface GlassSwitchProps {
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
  size?: "xs" | "sm" | "md";
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
const TARGET_TRACK_HEIGHT_BY_SIZE = {
  xs: 18,
  sm: 22,
  md: 28,
} as const;

/**
 * 中文注释：共享 glass 开关采用 switch 专用折射滤镜，
 * 不再复用通用 panel 材质，避免 thumb 的曲面和 specular 被抽象层抹平。
 */
export function GlassSwitch({
  checked,
  disabled = false,
  onChange: onChange,
  className: className,
  size = "md",
}: GlassSwitchProps) {
  const rawFilterId = useId();
  const filterId = `glass-switch-thumb-${rawFilterId.replace(/:/g, "")}`;
  const [canUseTrueGlass, setCanUseTrueGlass] = useState(false);
  const [isPressed, setIsPressed] = useState(false);
  const [isTransitioning, setIsTransitioning] = useState(false);
  const previousCheckedRef = useRef(checked);
  const targetTrackHeight = TARGET_TRACK_HEIGHT_BY_SIZE[size];
  const scaleRatio = targetTrackHeight / SOURCE_TRACK_HEIGHT;
  const trackWidth = Math.round(SOURCE_TRACK_WIDTH * scaleRatio);
  const trackHeight = targetTrackHeight;
  const thumbWidth = SOURCE_THUMB_WIDTH * scaleRatio;
  const thumbHeight = SOURCE_THUMB_HEIGHT * scaleRatio;
  const thumbRadius = SOURCE_THUMB_RADIUS * scaleRatio;
  const thumbOffsetX = SOURCE_THUMB_OFFSET_X * scaleRatio;
  const staticThumbTravelX = SOURCE_STATIC_THUMB_TRAVEL_X * scaleRatio;
  const thumbTravelX = SOURCE_THUMB_TRAVEL_X * scaleRatio;

  useEffect(() => {
    setCanUseTrueGlass(supportsTrueLiquidGlass());
  }, []);

  /**
   * 中文注释：首屏渲染时 previous_checked_ref 与当前值一致，
   * 不应把初始化当成一次开关动画，否则会错误显示 glass 覆盖层。
   */
  if (previousCheckedRef.current !== checked) {
    previousCheckedRef.current = checked;
    setIsTransitioning(true);
  }

  if (disabled && (isPressed || isTransitioning)) {
    setIsPressed(false);
    setIsTransitioning(false);
  }

  const showInteractionFilter = canUseTrueGlass
    && (isPressed || isTransitioning);

  return (
    <button
      aria-checked={checked}
      className={cn(
        "relative inline-flex shrink-0 items-center overflow-visible rounded-full transition-[background-color] duration-(--motion-duration-fast) ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(88,196,94,0.32)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent",
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
        className,
      )}
      onClick={() => {
        if (!disabled) {
          onChange(!checked);
        }
      }}
      onBlur={() => {
        setIsPressed(false);
      }}
      onKeyDown={(event) => {
        if (disabled) {
          return;
        }

        if (event.key === " " || event.key === "Enter") {
          setIsPressed(true);
        }
      }}
      onKeyUp={(event) => {
        if (event.key === " " || event.key === "Enter") {
          setIsPressed(false);
        }
      }}
      onPointerCancel={() => {
        setIsPressed(false);
      }}
      onPointerDown={(event) => {
        if (disabled) {
          return;
        }

        event.currentTarget.setPointerCapture(event.pointerId);
        setIsPressed(true);
      }}
      onPointerUp={(event) => {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
          event.currentTarget.releasePointerCapture(event.pointerId);
        }
        setIsPressed(false);
      }}
      role="switch"
      type="button"
      style={{
        width: `${trackWidth}px`,
        height: `${trackHeight}px`,
        backgroundColor: checked ? "rgba(59,191,78,0.93333)" : "rgba(198,201,210,0.82)",
      }}
    >
      {canUseTrueGlass ? (
        <svg
          aria-hidden="true"
          className="pointer-events-none absolute h-0 w-0 overflow-hidden"
          colorInterpolationFilters="sRGB"
          focusable="false"
        >
          <defs>
            <filter id={filterId}>
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
                width={thumbWidth}
                height={thumbHeight}
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
                width={thumbWidth}
                height={thumbHeight}
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
          width: `${thumbWidth}px`,
          height: `${thumbHeight}px`,
          marginLeft: `${thumbOffsetX}px`,
          top: `${trackHeight / 2}px`,
          borderRadius: `${thumbRadius}px`,
          backgroundColor: "rgb(255, 255, 255)",
          boxShadow: "0 4px 22px rgba(0, 0, 0, 0.1)",
          opacity: showInteractionFilter ? 0 : 1,
          transform: `translateX(${checked ? staticThumbTravelX : 0}px) translateY(-50%) scale(${SOURCE_STATIC_THUMB_SCALE})`,
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute rounded-full transition-[transform,opacity] duration-(--motion-duration-fast) ease-out will-change-transform"
        onTransitionEnd={(event) => {
          if (event.propertyName === "transform") {
            setIsTransitioning(false);
          }
        }}
        style={{
          width: `${thumbWidth}px`,
          height: `${thumbHeight}px`,
          marginLeft: `${thumbOffsetX}px`,
          top: `${trackHeight / 2}px`,
          borderRadius: `${thumbRadius}px`,
          backgroundColor: "rgba(255, 255, 255, 0.098)",
          boxShadow:
            "0 4px 22px rgba(0, 0, 0, 0.1), 2px 7px 24px rgba(0, 0, 0, 0.09) inset, -2px -7px 24px rgba(255, 255, 255, 0.09) inset",
          backdropFilter: showInteractionFilter ? `url(#${filterId})` : undefined,
          WebkitBackdropFilter: showInteractionFilter ? `url(#${filterId})` : undefined,
          opacity: showInteractionFilter ? 1 : 0,
          transform: `translateX(${checked ? thumbTravelX : 0}px) translateY(-50%) scale(${SOURCE_THUMB_SCALE})`,
        }}
      />
    </button>
  );
}
