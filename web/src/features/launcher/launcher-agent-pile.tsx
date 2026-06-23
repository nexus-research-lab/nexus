"use client";

import { useEffect, useMemo, useRef } from "react";
import Matter from "matter-js";

import {
  create_token_config,
  get_token_brand_style,
  hex_to_rgba,
} from "@/features/launcher/launcher-agent-pile-model";
import { cn } from "@/lib/utils";
import { SpotlightToken } from "@/types/app/launcher";

interface SpotlightTokenPileProps {
  class_name?: string;
  tokens: SpotlightToken[];
  current_agent_id: string | null;
  on_select_agent: (agent_id: string) => void;
}

export function AgentPile({
  class_name,
  tokens,
  current_agent_id,
  on_select_agent,
}: SpotlightTokenPileProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const tokenRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  const configs = useMemo(() => create_token_config(tokens, 560), [tokens]);
  const configByKey = useMemo(
    () => new Map(configs.map((config) => [config.key, config])),
    [configs],
  );
  const tokenByKey = useMemo(
    () => new Map(tokens.map((token) => [token.key, token])),
    [tokens],
  );

  useEffect(() => {
    const container = containerRef.current;
    if (!container || tokenByKey.size === 0) {
      return;
    }

    const { Engine, World, Bodies, Body } = Matter;
    const width = container.clientWidth || 560;
    const height = container.clientHeight;
    const engine = Engine.create({
      enableSleeping: true,
      gravity: { x: 0, y: 1.16, scale: 0.0034 },
      positionIterations: 8,
      velocityIterations: 6,
    });

    const bodyMap = new Map<string, Matter.Body>();
    const renderCache = new Map<string, { opacity: string; transform: string; zIndex: string }>();
    const timeoutIds: number[] = [];

    const ground = Bodies.rectangle(width / 2, height - 18, width + 120, 28, {
      isStatic: true,
      restitution: 0.16,
      friction: 0.84,
    });
    const leftWall = Bodies.rectangle(-18, height / 2, 36, height * 2, {
      isStatic: true,
      restitution: 0.12,
      friction: 0.9,
    });
    const rightWall = Bodies.rectangle(width + 18, height / 2, 36, height * 2, {
      isStatic: true,
      restitution: 0.12,
      friction: 0.9,
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

    World.add(engine.world, [ground, leftWall, rightWall, leftRamp, rightRamp]);

    configs.forEach((config) => {
      const token = tokenByKey.get(config.key);
      if (!token) {
        return;
      }

      const common = {
        restitution: 0.18,
        friction: 0.22,
        frictionAir: 0.012,
        density: 0.0014,
        sleepThreshold: 24,
        slop: 0.5,
      };

      const body =
        token.kind === "agent"
          ? Bodies.circle(config.spawn_x, config.spawn_y, config.size / 2, common)
          : Bodies.rectangle(config.spawn_x, config.spawn_y, config.size, config.size, {
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
    let disposed = false;
    let isDocumentVisible = document.visibilityState !== "hidden";
    let isInView = true;

    const update = (time: number) => {
      if (disposed || !isDocumentVisible || !isInView) {
        animationFrame = 0;
        previousTime = time;
        return;
      }

      // Matter 建议 delta 不超过 16.667ms，避免低帧率时积分不稳定。
      const delta = Math.min(time - previousTime, 1000 / 60);
      previousTime = time;
      Engine.update(engine, delta || 1000 / 60);

      // 检测所有动态 body 是否均已休眠；若是则停止 rAF，等待外部事件唤醒。
      // 必须至少有一个动态 body 才能判定"全部 sleep"，否则 tokens 还没 add 进来就会提前退出。
      const dynamicBodies = engine.world.bodies.filter((b) => !b.isStatic);
      const allAsleep =
        dynamicBodies.length > 0 &&
        dynamicBodies.every((b) => (b as Matter.Body & { isSleeping?: boolean }).isSleeping);

      let anyDirty = false;
      configs.forEach((config) => {
        const ref = tokenRefs.current[config.key];
        const body = bodyMap.get(config.key);
        if (!ref || !body) {
          return;
        }

        const nextOpacity = "1";
        // z-index 不能跟随掉落过程进入负值，否则 token 会在动画中被压到容器层后面，看起来像“消失”。
        const nextZIndex = `${1000 + Math.max(0, Math.round(body.position.y))}`;
        // 这里改回 2D transform，不再用 translate3d 强制提 GPU 合成层。
        // Token 数量不多时，2D 位移足够流畅，同时能显著减少层树里“一颗 token 一层”的情况。
        const nextTransform = `translate(${Math.round((body.position.x - config.size / 2) * 10) / 10}px, ${Math.round((body.position.y - config.size / 2) * 10) / 10}px) rotate(${Math.round(body.angle * 1000) / 1000}rad)`;
        const previousRender = renderCache.get(config.key);

        const changed =
          !previousRender ||
          previousRender.opacity !== nextOpacity ||
          previousRender.zIndex !== nextZIndex ||
          previousRender.transform !== nextTransform;

        if (changed) {
          anyDirty = true;
          ref.style.opacity = nextOpacity;
          ref.style.zIndex = nextZIndex;
          ref.style.transform = nextTransform;
          renderCache.set(config.key, { opacity: nextOpacity, transform: nextTransform, zIndex: nextZIndex });
        }
      });

      if (allAsleep && !anyDirty) {
        // 全部静止 — 停止循环，节省 CPU。交互事件（click/hover）通过 startAnimation 重启。
        animationFrame = 0;
        return;
      }

      animationFrame = window.requestAnimationFrame(update);
    };

    const stop_animation = () => {
      if (animationFrame !== 0) {
        window.cancelAnimationFrame(animationFrame);
        animationFrame = 0;
      }
    };

    const start_animation = () => {
      if (disposed || animationFrame !== 0 || !isDocumentVisible || !isInView) {
        return;
      }

      previousTime = performance.now();
      animationFrame = window.requestAnimationFrame(update);
    };

    const sync_animation_state = () => {
      if (isDocumentVisible && isInView) {
        start_animation();
        return;
      }

      stop_animation();
    };

    const intersectionObserver = new IntersectionObserver(
      ([entry]) => {
        isInView = entry?.isIntersecting ?? true;
        sync_animation_state();
      },
      { threshold: 0.05 },
    );
    intersectionObserver.observe(container);

    const handle_visibility_change = () => {
      isDocumentVisible = document.visibilityState !== "hidden";
      sync_animation_state();
    };
    document.addEventListener("visibilitychange", handle_visibility_change);

    sync_animation_state();

    return () => {
      disposed = true;
      timeoutIds.forEach((timeoutId) => window.clearTimeout(timeoutId));
      stop_animation();
      intersectionObserver.disconnect();
      document.removeEventListener("visibilitychange", handle_visibility_change);
      Matter.World.clear(engine.world, false);
      Matter.Engine.clear(engine);
    };
  }, [configs, tokenByKey]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "pointer-events-none relative z-0 mt-14 h-[286px] w-full max-w-[640px] overflow-hidden mask-[linear-gradient(180deg,transparent_0,black_14%,black_92%,transparent_100%)]",
        class_name,
      )}
    >
      <div className="pointer-events-none absolute bottom-[34px] left-1/2 h-[114px] w-[128%] -translate-x-1/2 rounded-[999px] border-t border-white/22 bg-[radial-gradient(circle_at_50%_8%,rgba(255,255,255,0.14),rgba(255,255,255,0.03)_28%,rgba(255,255,255,0)_62%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-[194px] h-px bg-[linear-gradient(90deg,rgba(255,255,255,0),rgba(255,255,255,0.1),rgba(255,255,255,0.3),rgba(255,255,255,0.1),rgba(255,255,255,0))]" />

      {tokens.map((token) => {
        const config = configByKey.get(token.key);
        if (!config) {
          return null;
        }

        const isActive = token.agent_id && token.agent_id === current_agent_id;
        const brandStyle = get_token_brand_style(token);

        return (
          <button
            key={token.key}
            ref={(node) => {
              tokenRefs.current[token.key] = node;
            }}
            className={cn(
              "pointer-events-auto absolute left-0 top-0 border opacity-0",
              token.kind === "agent" ? "rounded-full" : "rounded-[14px]",
              isActive && "ring-2 ring-white/80",
            )}
            data-token-kind={token.kind}
            onClick={() => token.agent_id && on_select_agent(token.agent_id)}
            style={{
              width: config.size,
              height: config.size,
              background: `linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(247,248,244,0.92) 100%)`,
              color: token.swatch.text,
              borderColor: hex_to_rgba("#ffffff", 0.46),
              boxShadow:
                token.kind === "agent"
                  ? `inset 0 1px 0 ${hex_to_rgba("#ffffff", 0.74)}, 0 16px 34px rgba(10,14,28,0.16), 0 0 18px ${hex_to_rgba(token.swatch.fill, 0.18)}`
                  : `inset 0 1px 0 ${hex_to_rgba("#ffffff", 0.68)}, 0 18px 38px rgba(10,14,28,0.18), 0 0 20px ${hex_to_rgba(token.swatch.fill, 0.2)}`,
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
                inset: brandStyle.inner_inset,
                borderRadius: brandStyle.inner_radius,
                background: `radial-gradient(circle at 28% 24%, ${hex_to_rgba("#ffffff", 0.32)} 0%, transparent 34%), linear-gradient(180deg, ${hex_to_rgba(token.swatch.fill, 0.88)} 0%, ${hex_to_rgba(token.swatch.fill, 1)} 100%)`,
                borderColor: hex_to_rgba(token.swatch.ring, 0.78),
                boxShadow: `inset 0 1px 0 ${hex_to_rgba("#ffffff", 0.34)}, inset 0 -3px 8px ${hex_to_rgba("#000000", 0.06)}`,
              }}
            />
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
                background: `linear-gradient(180deg, ${hex_to_rgba("#ffffff", brandStyle.gloss_opacity)} 0%, rgba(255,255,255,0) 100%)`,
              }}
            />
            <span
              className={cn(
                "relative z-10 flex h-full w-full flex-col items-center justify-center leading-none",
                brandStyle.rotation_class_name,
              )}
            >
              <span
                className={cn(
                  "font-black",
                  brandStyle.label_class_name,
                )}
                style={{
                  color: hex_to_rgba(token.swatch.text, 0.98),
                  textTransform: brandStyle.label_transform as "none" | "uppercase" | "capitalize",
                  textShadow: `0 1px 0 ${hex_to_rgba("#ffffff", 0.24)}, 0 2px 5px ${hex_to_rgba("#000000", 0.12)}`,
                }}
              >
                {token.label}
              </span>
              <span
                className={cn("mt-0.5 font-semibold uppercase", brandStyle.tag_class_name)}
                style={{
                  color: hex_to_rgba(token.swatch.text, brandStyle.tag_opacity),
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
