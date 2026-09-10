// INPUT: 玻璃盖板内的品牌内容。
// OUTPUT: 复用历史位移与高光贴图的玻璃材质及悬停弹性动效。
// POS: 仅持有视觉盖板，点击和可访问名称由外层入口负责。
import type { ReactNode } from "react";

import { useGlassMagnifierAnimation } from "./use-glass-magnifier-animation";
import { GlassMagnifierFilter } from "./glass-magnifier-filter";
import { useLiquidGlassFilterId, useSupportsTrueLiquidGlass } from "./use-liquid-glass-support";

export function GlassMagnifier({ children, underlay }: { children: ReactNode; underlay?: ReactNode }) {
  const filterId = useLiquidGlassFilterId("glass-magnifier");
  const animation = useGlassMagnifierAnimation();
  const supported = useSupportsTrueLiquidGlass();

  return (
    <span
      className="group/glass relative isolate inline-flex h-9 shrink-0 items-center rounded-[12px] px-2"
      ref={animation.rootRef}
      onPointerEnter={animation.onHoverStart}
      onPointerLeave={animation.onHoverEnd}
      onPointerCancel={animation.onHoverEnd}
      style={{ transform: animation.rootTransform }}
    >
      {underlay ? <span aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden rounded-[12px] opacity-0 transition-opacity duration-200 group-hover/glass:opacity-100">{underlay}</span> : null}
      {supported ? <GlassMagnifierFilter filterId={filterId} /> : null}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 rounded-[12px] ring-1 ring-black/5 dark:ring-white/10"
        style={{
          backdropFilter: supported ? `url(#${filterId})` : "blur(16px)",
          WebkitBackdropFilter: supported ? `url(#${filterId})` : "blur(16px)",
          backgroundColor: supported ? "rgba(255,255,255,0.01)" : "color-mix(in srgb, var(--surface-panel-background) 24%, transparent)",
          boxShadow: "0 2px 6px rgba(0,0,0,0.06), inset 0 1px 8px rgba(0,0,0,0.05), inset 0 -1px 8px rgba(255,255,255,0.3)",
        }}
      />
      <span className="relative shrink-0" ref={animation.contentRef} style={{ transform: animation.contentTransform }}>{children}</span>
    </span>
  );
}
