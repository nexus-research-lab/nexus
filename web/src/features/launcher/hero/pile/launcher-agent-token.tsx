import { cn } from "@/lib/utils";
import type { SpotlightToken } from "@/types/app/launcher";

import {
  getTokenBrandStyle,
  hexToRgba,
  type TokenPhysicsConfig,
} from "./launcher-agent-pile-model";

interface LauncherAgentTokenProps {
  bindElement: (element: HTMLElement | null) => void;
  config: TokenPhysicsConfig;
  isActive: boolean;
  onSelectAgent: (agentId: string) => void;
  token: SpotlightToken;
}

interface TokenShapeStyle {
  glossRadiusClassName: string;
  glossTop: string;
  outerRadiusClassName: string;
  shadow: (fill: string) => string;
}

const SHAPE_BY_KIND: Readonly<Record<SpotlightToken["kind"], TokenShapeStyle>> = {
  agent: {
    glossRadiusClassName: "rounded-full",
    glossTop: "18%",
    outerRadiusClassName: "rounded-full",
    shadow: (fill) =>
      `inset 0 1px 0 ${hexToRgba("#ffffff", 0.74)}, 0 16px 34px rgba(10,14,28,0.16), 0 0 18px ${hexToRgba(fill, 0.18)}`,
  },
  room: {
    glossRadiusClassName: "rounded-[999px]",
    glossTop: "16%",
    outerRadiusClassName: "rounded-[14px]",
    shadow: (fill) =>
      `inset 0 1px 0 ${hexToRgba("#ffffff", 0.68)}, 0 18px 38px rgba(10,14,28,0.18), 0 0 20px ${hexToRgba(fill, 0.2)}`,
  },
};

export function LauncherAgentToken({
  bindElement,
  config,
  isActive,
  onSelectAgent,
  token,
}: LauncherAgentTokenProps) {
  const shape = SHAPE_BY_KIND[token.kind];
  const content = (
    <TokenFace shape={shape} token={token} />
  );
  const commonClassName = cn(
    "absolute left-0 top-0 border opacity-0",
    shape.outerRadiusClassName,
    isActive && "ring-2 ring-white/80",
  );
  const style = {
    background: "linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(247,248,244,0.92) 100%)",
    borderColor: hexToRgba("#ffffff", 0.46),
    boxShadow: shape.shadow(token.swatch.fill),
    color: token.swatch.text,
    height: config.size,
    width: config.size,
  };
  const agentId = token.agent_id;

  return agentId ? (
    <button
      className={cn("pointer-events-auto", commonClassName)}
      data-token-kind={token.kind}
      onClick={() => onSelectAgent(agentId)}
      ref={bindElement}
      style={style}
      type="button"
    >
      {content}
    </button>
  ) : (
    <div
      aria-hidden="true"
      className={cn("pointer-events-none", commonClassName)}
      data-token-kind={token.kind}
      ref={bindElement}
      style={style}
    >
      {content}
    </div>
  );
}

function TokenFace({
  shape,
  token,
}: {
  shape: TokenShapeStyle;
  token: SpotlightToken;
}) {
  const brand = getTokenBrandStyle(token);
  return (
    <>
      <span
        aria-hidden="true"
        className="pointer-events-none absolute border"
        style={{
          background: `radial-gradient(circle at 28% 24%, ${hexToRgba("#ffffff", 0.32)} 0%, transparent 34%), linear-gradient(180deg, ${hexToRgba(token.swatch.fill, 0.88)} 0%, ${hexToRgba(token.swatch.fill, 1)} 100%)`,
          borderColor: hexToRgba(token.swatch.ring, 0.78),
          borderRadius: brand.innerRadius,
          boxShadow: `inset 0 1px 0 ${hexToRgba("#ffffff", 0.34)}, inset 0 -3px 8px ${hexToRgba("#000000", 0.06)}`,
          inset: brand.innerInset,
        }}
      />
      <span
        aria-hidden="true"
        className={cn("pointer-events-none absolute", shape.glossRadiusClassName)}
        style={{
          background: `linear-gradient(180deg, ${hexToRgba("#ffffff", brand.glossOpacity)} 0%, rgba(255,255,255,0) 100%)`,
          height: "22%",
          left: "16%",
          right: "16%",
          top: shape.glossTop,
        }}
      />
      <span className={cn(
        "relative z-10 flex h-full w-full flex-col items-center justify-center leading-none",
        brand.rotationClassName,
      )}>
        <span
          className={cn("font-black", brand.labelClassName)}
          style={{
            color: hexToRgba(token.swatch.text, 0.98),
            textShadow: `0 1px 0 ${hexToRgba("#ffffff", 0.24)}, 0 2px 5px ${hexToRgba("#000000", 0.12)}`,
            textTransform: brand.labelTransform,
          }}
        >
          {token.label}
        </span>
        <span
          className={cn("mt-0.5 font-semibold uppercase", brand.tagClassName)}
          style={{ color: hexToRgba(token.swatch.text, brand.tagOpacity) }}
        >
          {brand.tag}
        </span>
      </span>
    </>
  );
}
