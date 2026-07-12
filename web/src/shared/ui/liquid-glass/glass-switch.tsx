import type { CSSProperties } from "react";

import { cn } from "@/shared/ui/class-name";

import { GlassSwitchFilter } from "./glass-switch-filter";
import { useGlassSwitchInteraction } from "./use-glass-switch-interaction";
import {
  useLiquidGlassFilterId,
  useSupportsTrueLiquidGlass,
} from "./use-liquid-glass-support";

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
const TARGET_TRACK_HEIGHT_BY_SIZE = {
  xs: 18,
  sm: 22,
  md: 28,
} as const;

interface GlassSwitchPresentation {
  dynamicThumbStyle: CSSProperties;
  showFilterResource: boolean;
  staticThumbStyle: CSSProperties;
  thumbHeight: number;
  thumbWidth: number;
  trackStyle: CSSProperties;
}

function getGlassSwitchPresentation({
  canUseTrueGlass,
  checked,
  filterId,
  isPressed,
  isTransitioning,
  size,
}: {
  canUseTrueGlass: boolean;
  checked: boolean;
  filterId: string;
  isPressed: boolean;
  isTransitioning: boolean;
  size: NonNullable<GlassSwitchProps["size"]>;
}): GlassSwitchPresentation {
  const trackHeight = TARGET_TRACK_HEIGHT_BY_SIZE[size];
  const scaleRatio = trackHeight / SOURCE_TRACK_HEIGHT;
  const trackWidth = Math.round(SOURCE_TRACK_WIDTH * scaleRatio);
  const thumbWidth = SOURCE_THUMB_WIDTH * scaleRatio;
  const thumbHeight = SOURCE_THUMB_HEIGHT * scaleRatio;
  const thumbRadius = SOURCE_THUMB_RADIUS * scaleRatio;
  const thumbOffsetX = SOURCE_THUMB_OFFSET_X * scaleRatio;
  const staticThumbTravelX = SOURCE_STATIC_THUMB_TRAVEL_X * scaleRatio;
  const thumbTravelX = SOURCE_THUMB_TRAVEL_X * scaleRatio;
  const showInteractionFilter = canUseTrueGlass && (isPressed || isTransitioning);
  const filterValue = showInteractionFilter ? `url(#${filterId})` : undefined;

  return {
    dynamicThumbStyle: {
      WebkitBackdropFilter: filterValue,
      backdropFilter: filterValue,
      backgroundColor: "rgba(255, 255, 255, 0.098)",
      borderRadius: `${thumbRadius}px`,
      boxShadow: "0 4px 22px rgba(0, 0, 0, 0.1), 2px 7px 24px rgba(0, 0, 0, 0.09) inset, -2px -7px 24px rgba(255, 255, 255, 0.09) inset",
      height: `${thumbHeight}px`,
      marginLeft: `${thumbOffsetX}px`,
      opacity: showInteractionFilter ? 1 : 0,
      top: `${trackHeight / 2}px`,
      transform: `translateX(${checked ? thumbTravelX : 0}px) translateY(-50%) scale(${SOURCE_THUMB_SCALE})`,
      width: `${thumbWidth}px`,
    },
    showFilterResource: canUseTrueGlass,
    staticThumbStyle: {
      backgroundColor: "rgb(255, 255, 255)",
      borderRadius: `${thumbRadius}px`,
      boxShadow: "0 4px 22px rgba(0, 0, 0, 0.1)",
      height: `${thumbHeight}px`,
      marginLeft: `${thumbOffsetX}px`,
      opacity: showInteractionFilter ? 0 : 1,
      top: `${trackHeight / 2}px`,
      transform: `translateX(${checked ? staticThumbTravelX : 0}px) translateY(-50%) scale(${SOURCE_STATIC_THUMB_SCALE})`,
      width: `${thumbWidth}px`,
    },
    thumbHeight,
    thumbWidth,
    trackStyle: {
      backgroundColor: checked ? "rgba(59,191,78,0.93333)" : "rgba(198,201,210,0.82)",
      height: `${trackHeight}px`,
      width: `${trackWidth}px`,
    },
  };
}

/** 开关保留专用折射几何，避免通用面板材质抹平 thumb 的曲面和高光。 */
export function GlassSwitch({
  checked,
  disabled = false,
  onChange,
  className,
  size = "md",
}: GlassSwitchProps) {
  const filterId = useLiquidGlassFilterId("glass-switch-thumb");
  const canUseTrueGlass = useSupportsTrueLiquidGlass();
  const interaction = useGlassSwitchInteraction({ checked, disabled, onChange });
  const presentation = getGlassSwitchPresentation({
    canUseTrueGlass,
    checked,
    filterId,
    isPressed: interaction.isPressed,
    isTransitioning: interaction.isTransitioning,
    size,
  });

  return (
    <button
      {...interaction.buttonHandlers}
      aria-checked={checked}
      className={cn(
        "relative inline-flex shrink-0 items-center overflow-visible rounded-full transition-[background-color] duration-(--motion-duration-fast) ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(88,196,94,0.32)] focus-visible:ring-offset-2 focus-visible:ring-offset-transparent",
        disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
        className,
      )}
      role="switch"
      type="button"
      style={presentation.trackStyle}
    >
      {presentation.showFilterResource ? (
        <GlassSwitchFilter
          filterId={filterId}
          height={presentation.thumbHeight}
          width={presentation.thumbWidth}
        />
      ) : null}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute rounded-full transition-[transform,opacity] duration-(--motion-duration-fast) ease-out will-change-transform"
        style={presentation.staticThumbStyle}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute rounded-full transition-[transform,opacity] duration-(--motion-duration-fast) ease-out will-change-transform"
        onTransitionEnd={interaction.onThumbTransitionEnd}
        style={presentation.dynamicThumbStyle}
      />
    </button>
  );
}
