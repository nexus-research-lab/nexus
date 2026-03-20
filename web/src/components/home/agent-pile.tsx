"use client";

import { useEffect, useMemo, useRef } from "react";
import Matter from "matter-js";

import { cn } from "@/lib/utils";

export type SpotlightToken = {
  key: string;
  label: string;
  agentId: string | null;
  kind: "agent" | "room";
  swatch: {
    fill: string;
    text: string;
    ring: string;
  };
};

interface SpotlightTokenPileProps {
  tokens: SpotlightToken[];
  currentAgentId: string | null;
  onSelectAgent: (agentId: string) => void;
}

type TokenPhysicsConfig = {
  key: string;
  size: number;
  radius: number;
  spawnX: number;
  spawnY: number;
  angle: number;
  delay: number;
};

type TokenBrandStyle = {
  labelClassName: string;
  labelTransform: string;
  tag: string;
  tagClassName: string;
  tagOpacity: number;
  rotationClassName: string;
  innerInset: number;
  innerRadius: string;
  accentOpacity: number;
  glossOpacity: number;
  fold: boolean;
  stacked: boolean;
  ring: boolean;
};

function createTokenConfig(tokens: SpotlightToken[], width: number): TokenPhysicsConfig[] {
  const horizontalPadding = 108;
  return tokens.map((token, index) => {
    const baseSize = token.kind === "agent" ? 40 : 44;
    const size = baseSize + Math.round(Math.random() * 12);
    return {
      key: token.key,
      size,
      radius: token.kind === "agent" ? size / 2 : Math.max(12, Math.round(size * 0.28)),
      spawnX:
        horizontalPadding + Math.random() * Math.max(width - horizontalPadding * 2, 72),
      spawnY: -180 - Math.random() * 240 - index * 14,
      angle: ((Math.random() * 36 - 18) * Math.PI) / 180,
      delay: 40 + index * 55,
    };
  });
}

function hexToRgba(hex: string, alpha: number) {
  const normalized = hex.replace("#", "");
  const value =
    normalized.length === 3
      ? normalized
        .split("")
        .map((item) => `${item}${item}`)
        .join("")
      : normalized;
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

function getLabelSize(label: string) {
  if (label.length >= 3) {
    return "text-[10px]";
  }
  return "text-[12px]";
}

function hashString(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}

function getTokenBrandStyle(token: SpotlightToken): TokenBrandStyle {
  const hash = hashString(token.key);
  const variant = hash % 5;

  if (variant === 0) {
    return {
      labelClassName: token.label.length >= 3 ? "text-[9px] tracking-[-0.03em]" : "text-[13px] tracking-[-0.08em]",
      labelTransform: "none",
      tag: token.kind === "agent" ? "core" : "room",
      tagClassName: "text-[6px] tracking-[0.2em]",
      tagOpacity: 0.62,
      rotationClassName: "",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "12px",
      accentOpacity: 0.2,
      glossOpacity: 0.38,
      fold: false,
      stacked: false,
      ring: true,
    };
  }

  if (variant === 1) {
    return {
      labelClassName: token.label.length >= 3 ? "text-[8px] tracking-[0.04em]" : "text-[12px] tracking-[0.08em]",
      labelTransform: "uppercase",
      tag: token.kind === "agent" ? "lab" : "sync",
      tagClassName: "text-[6px] tracking-[0.24em]",
      tagOpacity: 0.54,
      rotationClassName: "rotate-[-4deg]",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "11px",
      accentOpacity: 0.26,
      glossOpacity: 0.32,
      fold: token.kind === "room",
      stacked: false,
      ring: false,
    };
  }

  if (variant === 2) {
    return {
      labelClassName: token.label.length >= 3 ? "text-[10px] tracking-[-0.08em]" : "text-[14px] tracking-[-0.1em]",
      labelTransform: "none",
      tag: token.kind === "agent" ? "net" : "grid",
      tagClassName: "text-[6px] tracking-[0.16em]",
      tagOpacity: 0.58,
      rotationClassName: token.kind === "room" ? "rotate-[-8deg]" : "",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "12px",
      accentOpacity: 0.18,
      glossOpacity: 0.34,
      fold: false,
      stacked: token.kind === "room",
      ring: false,
    };
  }

  if (variant === 3) {
    return {
      labelClassName: getLabelSize(token.label),
      labelTransform: "capitalize",
      tag: token.kind === "agent" ? "ai" : "hub",
      tagClassName: "text-[6px] tracking-[0.28em]",
      tagOpacity: 0.48,
      rotationClassName: "rotate-[3deg]",
      innerInset: 1.5,
      innerRadius: token.kind === "agent" ? "9999px" : "13px",
      accentOpacity: 0.24,
      glossOpacity: 0.3,
      fold: hash % 2 === 0,
      stacked: false,
      ring: false,
    };
  }

  return {
    labelClassName: token.label.length >= 3 ? "text-[8px] tracking-[0.12em]" : "text-[11px] tracking-[0.16em]",
    labelTransform: "uppercase",
    tag: token.kind === "agent" ? "os" : "flow",
    tagClassName: "text-[5px] tracking-[0.3em]",
    tagOpacity: 0.42,
    rotationClassName: token.kind === "room" ? "rotate-[6deg]" : "rotate-[-2deg]",
    innerInset: 2.5,
    innerRadius: token.kind === "agent" ? "9999px" : "10px",
    accentOpacity: 0.22,
    glossOpacity: 0.26,
    fold: false,
    stacked: true,
    ring: hash % 2 === 1,
  };
}

export function AgentPile({
  tokens,
  currentAgentId,
  onSelectAgent,
}: SpotlightTokenPileProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const tokenRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  const physicsSeed = useMemo(() => tokens.map((token) => token.key).join("|"), [tokens]);
  const configs = useMemo(() => createTokenConfig(tokens, 560), [physicsSeed, tokens]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || tokens.length === 0) {
      return;
    }

    const { Engine, World, Bodies, Body } = Matter;
    const width = container.clientWidth || 560;
    const height = container.clientHeight;
    const engine = Engine.create({
      gravity: { x: 0, y: 1.16, scale: 0.0034 },
      positionIterations: 10,
      velocityIterations: 9,
    });

    const bodyMap = new Map<string, Matter.Body>();
    const timeoutIds: number[] = [];

    const ground = Bodies.rectangle(width / 2, height - 18, width + 120, 28, {
      isStatic: true,
      restitution: 0.16,
      friction: 0.84,
    });
    const leftRamp = Bodies.rectangle(-42, height / 2, 180, height * 2, {
      isStatic: true,
      angle: -0.16,
      restitution: 0.1,
      friction: 0.88,
    });
    const rightRamp = Bodies.rectangle(width + 42, height / 2, 180, height * 2, {
      isStatic: true,
      angle: 0.16,
      restitution: 0.1,
      friction: 0.88,
    });

    World.add(engine.world, [ground, leftRamp, rightRamp]);

    configs.forEach((config) => {
      const token = tokens.find((item) => item.key === config.key);
      if (!token) {
        return;
      }

      const common = {
        restitution: 0.18,
        friction: 0.22,
        frictionAir: 0.012,
        density: 0.0014,
        slop: 0.5,
      };

      const body =
        token.kind === "agent"
          ? Bodies.circle(config.spawnX, config.spawnY, config.size / 2, common)
          : Bodies.rectangle(config.spawnX, config.spawnY, config.size, config.size, {
            ...common,
            chamfer: { radius: config.radius },
          });

      Body.setAngle(body, config.angle);
      Body.setVelocity(body, {
        x: Math.random() * 2.6 - 1.3,
        y: 3.8 + Math.random() * 1.8,
      });
      Body.setAngularVelocity(body, (Math.random() * 0.06 - 0.03) * (token.kind === "room" ? 1.2 : 0.8));
      bodyMap.set(config.key, body);

      const timeoutId = window.setTimeout(() => {
        World.add(engine.world, body);
      }, config.delay);
      timeoutIds.push(timeoutId);
    });

    let animationFrame = 0;
    let previousTime = performance.now();

    const update = (time: number) => {
      const delta = Math.min(time - previousTime, 1000 / 30);
      previousTime = time;
      Engine.update(engine, delta || 1000 / 60);

      tokens.forEach((token) => {
        const ref = tokenRefs.current[token.key];
        const body = bodyMap.get(token.key);
        if (!ref || !body) {
          return;
        }

        const config = configs.find((item) => item.key === token.key);
        if (!config) {
          return;
        }

        ref.style.opacity = "1";
        ref.style.zIndex = `${Math.round(body.position.y)}`;
        ref.style.transform = `translate3d(${body.position.x - config.size / 2}px, ${body.position.y - config.size / 2}px, 0) rotate(${body.angle}rad)`;
      });

      animationFrame = window.requestAnimationFrame(update);
    };

    animationFrame = window.requestAnimationFrame(update);

    return () => {
      timeoutIds.forEach((timeoutId) => window.clearTimeout(timeoutId));
      window.cancelAnimationFrame(animationFrame);
      Matter.World.clear(engine.world, false);
      Matter.Engine.clear(engine);
    };
  }, [configs, physicsSeed, tokens]);

  return (
    <div
      ref={containerRef}
      className="relative mt-20 h-[238px] w-full max-w-[560px] overflow-hidden [mask-image:linear-gradient(90deg,transparent_0,black_8%,black_92%,transparent_100%)]"
    >
      <div className="pointer-events-none absolute left-0 right-0 top-[176px] h-px bg-[linear-gradient(90deg,rgba(58,61,56,0),rgba(58,61,56,0.18),rgba(58,61,56,0.28),rgba(58,61,56,0.18),rgba(58,61,56,0))]" />

      {tokens.map((token) => {
        const config = configs.find((item) => item.key === token.key);
        if (!config) {
          return null;
        }

        const isActive = token.agentId && token.agentId === currentAgentId;
        const brandStyle = getTokenBrandStyle(token);

        return (
          <button
            key={token.key}
            ref={(node) => {
              tokenRefs.current[token.key] = node;
            }}
            className={cn(
              "absolute left-0 top-0 overflow-hidden border opacity-0 will-change-transform transition-[filter] duration-200 hover:brightness-[1.04]",
              token.kind === "agent" ? "rounded-full" : "rounded-[14px]",
              isActive && "ring-2 ring-white/80",
            )}
            data-token-kind={token.kind}
            onClick={() => token.agentId && onSelectAgent(token.agentId)}
            style={{
              width: config.size,
              height: config.size,
              background: `linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(247,248,244,0.92) 100%)`,
              color: token.swatch.text,
              borderColor: hexToRgba("#ffffff", 0.46),
              boxShadow:
                token.kind === "agent"
                  ? `inset 0 1px 0 ${hexToRgba("#ffffff", 0.74)}, 0 14px 28px rgba(53, 59, 50, 0.13), 0 2px 8px ${hexToRgba(token.swatch.fill, 0.13)}`
                  : `inset 0 1px 0 ${hexToRgba("#ffffff", 0.68)}, 0 16px 30px rgba(53, 59, 50, 0.14), 0 2px 9px ${hexToRgba(token.swatch.fill, 0.14)}`,
            }}
            type="button"
          >
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute border",
                token.kind === "agent" ? "rounded-full" : "rounded-[11px]",
              )}
              style={{
                inset: brandStyle.innerInset,
                borderRadius: brandStyle.innerRadius,
                background: `radial-gradient(circle at 28% 24%, ${hexToRgba("#ffffff", 0.32)} 0%, transparent 34%), linear-gradient(180deg, ${hexToRgba(token.swatch.fill, 0.88)} 0%, ${hexToRgba(token.swatch.fill, 1)} 100%)`,
                borderColor: hexToRgba(token.swatch.ring, 0.78),
                boxShadow: `inset 0 1px 0 ${hexToRgba("#ffffff", 0.34)}, inset 0 -3px 8px ${hexToRgba("#000000", 0.06)}`,
              }}
            />
            {brandStyle.stacked && (
              <span
                aria-hidden="true"
                className={cn(
                  "pointer-events-none absolute border",
                  token.kind === "agent" ? "rounded-full" : "rounded-[12px]",
                )}
                style={{
                  inset: 4,
                  transform: "translate(2px, 3px)",
                  borderColor: hexToRgba(token.swatch.ring, 0.28),
                  background: hexToRgba(token.swatch.fill, 0.16),
                  zIndex: 0,
                }}
              />
            )}
            {brandStyle.fold && token.kind === "room" && (
              <span
                aria-hidden="true"
                className="pointer-events-none absolute right-[7%] top-[7%] h-[26%] w-[26%] overflow-hidden rounded-[8px]"
                style={{
                  background: `linear-gradient(135deg, ${hexToRgba("#ffffff", 0.84)} 0%, ${hexToRgba(token.swatch.fill, 0.16)} 58%, transparent 58%)`,
                  boxShadow: `inset 0 1px 0 ${hexToRgba("#ffffff", 0.46)}`,
                }}
              >
                <span
                  className="absolute inset-0"
                  style={{
                    background: `linear-gradient(135deg, transparent 0 48%, ${hexToRgba("#000000", 0.08)} 52%, transparent 60%)`,
                  }}
                />
              </span>
            )}
            {brandStyle.ring && (
              <span
                aria-hidden="true"
                className={cn(
                  "pointer-events-none absolute border",
                  token.kind === "agent" ? "rounded-full" : "rounded-[10px]",
                )}
                style={{
                  inset: token.kind === "agent" ? "24%" : "22%",
                  borderColor: hexToRgba(token.swatch.text, 0.28),
                }}
              />
            )}
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute",
                token.kind === "agent" ? "rounded-full" : "rounded-[999px]",
              )}
              style={{
                left: "16%",
                right: "16%",
                top: token.kind === "agent" ? "18%" : "16%",
                height: "22%",
                background: `linear-gradient(180deg, ${hexToRgba("#ffffff", brandStyle.glossOpacity)} 0%, rgba(255,255,255,0) 100%)`,
                filter: "blur(0.8px)",
              }}
            />
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute blur-[0.8px]",
                token.kind === "agent" ? "rounded-full" : "rounded-[999px]",
              )}
              style={{
                left: "18%",
                top: "18%",
                height: "26%",
                width: "42%",
                background: "linear-gradient(180deg, rgba(255,255,255,0.42), rgba(255,255,255,0))",
              }}
            />
            <span
              aria-hidden="true"
              className={cn(
                "pointer-events-none absolute",
                token.kind === "agent" ? "rounded-full" : "rounded-[10px]",
              )}
              style={{
                inset: token.kind === "agent" ? "10%" : "12%",
                background: `radial-gradient(circle at 50% 56%, transparent 0%, transparent 58%, ${hexToRgba(token.swatch.text, brandStyle.accentOpacity)} 100%)`,
                opacity: 0.55,
              }}
            />
            <span
              className={cn(
                "relative z-10 flex h-full w-full flex-col items-center justify-center leading-none",
                brandStyle.rotationClassName,
              )}
            >
              <span
                className={cn(
                  "font-black",
                  brandStyle.labelClassName,
                )}
                style={{
                  color: hexToRgba(token.swatch.text, 0.98),
                  textTransform: brandStyle.labelTransform as "none" | "uppercase" | "capitalize",
                  textShadow: `0 1px 0 ${hexToRgba("#ffffff", 0.24)}, 0 2px 5px ${hexToRgba("#000000", 0.12)}`,
                }}
              >
                {token.label}
              </span>
              <span
                className={cn("mt-0.5 font-semibold uppercase", brandStyle.tagClassName)}
                style={{
                  color: hexToRgba(token.swatch.text, brandStyle.tagOpacity),
                }}
              >
                {brandStyle.tag}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
