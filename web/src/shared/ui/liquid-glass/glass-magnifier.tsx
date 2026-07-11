import {
  type CSSProperties,
  type ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";

import { GlassMagnifierFilter } from "./glass-magnifier-filter";
import { useGlassMagnifierAnimation } from "./use-glass-magnifier-animation";
import {
  useLiquidGlassFilterId,
  useSupportsTrueLiquidGlass,
} from "./use-liquid-glass-support";

interface GlassMagnifierProps {
  children?: ReactNode;
  className?: string;
  contentClassName?: string;
  height?: number;
  underlay?: ReactNode;
  width?: number;
}

const BASE_LENS_RADIUS = 75;

function buildGlassSurfaceStyle(filterId: string | null): CSSProperties {
  return {
    borderRadius: `${BASE_LENS_RADIUS}px`,
    boxShadow: "rgba(0, 0, 0, 0.16) 0px 4px 9px, rgba(0, 0, 0, 0.2) 0px 2px 24px inset, rgba(255, 255, 255, 0.2) 0px -2px 24px inset",
    transform: "translateZ(0px)",
    backdropFilter: filterId ? `url(#${filterId})` : "blur(16px)",
    WebkitBackdropFilter: filterId ? `url(#${filterId})` : "blur(16px)",
    backgroundColor: filterId ? "rgba(255, 255, 255, 0.01)" : "color-mix(in srgb, var(--surface-panel-background) 72%, transparent)",
  };
}

export function GlassMagnifier({
  children,
  className,
  contentClassName,
  height = 36,
  underlay,
  width = 58,
}: GlassMagnifierProps) {
  const filterId = useLiquidGlassFilterId("glass-magnifier");
  const canUseTrueGlass = useSupportsTrueLiquidGlass();
  const animation = useGlassMagnifierAnimation({ height, width });

  return (
    <div
      className={cn("relative isolate shrink-0 cursor-grab select-none active:cursor-grabbing", className)}
      onPointerCancel={animation.onHoverEnd}
      onPointerEnter={animation.onHoverStart}
      onPointerLeave={animation.onHoverEnd}
      ref={animation.rootRef}
      style={{
        height: `${height}px`,
        touchAction: "none",
        transform: animation.rootTransform,
        transformOrigin: "50% 50%",
        userSelect: "none",
        willChange: "transform",
        width: `${width}px`,
      }}
    >
      {underlay ? (
        <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[999px]">
          {underlay}
        </div>
      ) : null}

      {canUseTrueGlass ? (
        <GlassMagnifierFilter
          filterId={filterId}
          height={animation.sourceSize.height}
          width={animation.sourceSize.width}
        />
      ) : null}

      <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[999px]">
        <div
          className="absolute left-0 top-0 origin-top-left ring-1 ring-black/10 dark:ring-white/10"
          ref={animation.shellRef}
          style={{
            ...buildGlassSurfaceStyle(canUseTrueGlass ? filterId : null),
            height: `${animation.sourceSize.height}px`,
            transform: animation.idleTransform,
            width: `${animation.sourceSize.width}px`,
            willChange: "transform",
          }}
        />
      </div>

      {children ? (
        <div
          className={cn(
            "pointer-events-none absolute inset-0 z-10 flex items-center justify-center",
            contentClassName,
          )}
          ref={animation.contentRef}
          style={{
            transform: animation.contentTransform,
            transformOrigin: "50% 50%",
            willChange: "transform",
          }}
        >
          {children}
        </div>
      ) : null}
    </div>
  );
}
