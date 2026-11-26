// INPUT: 玻璃盖板内的品牌内容。
// OUTPUT: 复用历史位移与高光贴图的玻璃材质及悬停弹性动效。
// POS: 仅持有视觉盖板，点击和可访问名称由外层入口负责。
import { useEffect, useState, type ReactNode, type RefObject } from "react";

import { useGlassMagnifierAnimation } from "./use-glass-magnifier-animation";
import { GlassMagnifierFilter } from "./glass-magnifier-filter";
import { useLiquidGlassFilterId, useSupportsTrueLiquidGlass } from "./use-liquid-glass-support";

export function GlassMagnifier({ children, underlay }: { children: ReactNode; underlay?: ReactNode }) {
  const filterId = useLiquidGlassFilterId("glass-magnifier");
  const animation = useGlassMagnifierAnimation();
  const supported = useSupportsTrueLiquidGlass();
  const layerKey = useGlassLayerSizeKey(animation.rootRef);

  return (
    <span
      className="group/glass relative isolate inline-flex h-9 shrink-0 items-center rounded-(--radius-surface-lg) px-2"
      ref={animation.rootRef}
      onPointerEnter={animation.onHoverStart}
      onPointerLeave={animation.onHoverEnd}
      onPointerCancel={animation.onHoverEnd}
      style={{ transform: animation.rootTransform }}
    >
      {underlay ? <span aria-hidden="true" key={`underlay-${layerKey}`} className="pointer-events-none absolute inset-0 overflow-hidden rounded-(--radius-surface-lg) opacity-0 transition-opacity duration-(--motion-duration-fast) group-hover/glass:opacity-100">{underlay}</span> : null}
      {supported ? <GlassMagnifierFilter filterId={filterId} /> : null}
      <span
        aria-hidden="true"
        key={`backdrop-${layerKey}`}
        className="pointer-events-none absolute inset-0 rounded-(--radius-surface-lg) ring-1 ring-black/5 dark:ring-white/10"
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

/**
 * WebKit（macOS 桌面宿主）不会在盖板变宽后重绘 backdrop-filter 与蒙版层：字标在 Panchang 加载前按回退字体排版，
 * 字体到位后盖板变宽，旧的玻璃与彩光层仍停在原来的宽度，只盖住左半边。尺寸变化时换 key 重建这两层，让它们按新尺寸铺满。
 */
function useGlassLayerSizeKey(rootRef: RefObject<HTMLSpanElement | null>): string {
  const [sizeKey, setSizeKey] = useState("");
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) {
        return;
      }
      // 只看布局尺寸：悬停动效的 transform 不改变 contentRect，不会触发重建。
      const { height, width } = entry.contentRect;
      setSizeKey(`${Math.round(width)}x${Math.round(height)}`);
    });
    observer.observe(root);
    return () => observer.disconnect();
  }, [rootRef]);
  return sizeKey;
}
