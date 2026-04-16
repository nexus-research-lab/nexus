/**
 * =====================================================
 * @File   : home-ascii-hero.tsx
 * @Date   : 2026-04-11 22:47
 * @Author : leemysw
 * 2026-04-11 22:47   Create
 * =====================================================
 */

"use client";

import { useEffect, useRef } from "react";

import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { useTheme } from "@/shared/theme/theme-context";

const ASCII_CHARS = ".:+-=*#@&~<>{}[]|/\\";
const MOBILE_ASCII_CHARS = "01";
const HERO_LABEL = "nexus";
const DEFAULT_HERO_INK = "#516dff";
const DEFAULT_CLOCK_INK = "rgba(32, 45, 62, 0.88)";

interface HomeAsciiHeroProps {
  agent_count: number;
  room_count: number;
}

interface AsciiParticle {
  x: number;
  y: number;
  tx: number;
  ty: number;
  vx: number;
  vy: number;
  char: string;
  alpha: number;
  target_alpha: number;
  is_text: boolean;
  phase: number;
  delay: number;
}

function pick(charset: string) {
  return charset[Math.floor(Math.random() * charset.length)] ?? ".";
}

function pad2(n: number) {
  return n.toString().padStart(2, "0");
}

export function HomeAsciiHero({ agent_count, room_count }: HomeAsciiHeroProps) {
  const { theme } = useTheme();
  const section_ref = useRef<HTMLDivElement | null>(null);
  const canvas_ref = useRef<HTMLCanvasElement | null>(null);
  const prefers_reduced_motion = usePrefersReducedMotion();

  useEffect(() => {
    const section = section_ref.current;
    const canvas = canvas_ref.current;
    if (!section || !canvas || prefers_reduced_motion) {
      return;
    }

    const ctx = canvas.getContext("2d");
    if (!ctx) {
      return;
    }
    const hero_canvas = canvas;
    const hero_ctx = ctx;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const mobile_q = window.matchMedia("(max-width: 600px)");

    let particles: AsciiParticle[] = [];
    let width = 0;
    let height = 0;
    let glyph_size = 6;
    let glyph_font = "";
    let influence_radius = 100;
    let influence_radius_sq = influence_radius * influence_radius;
    let influence_force = 3;
    let frame_id = 0;
    let pointer_x = -9999;
    let pointer_y = -9999;
    let is_dead = false;
    let is_mobile = false;
    let clock_pad_x = 22;
    let clock_pad_y = 18;
    let clock_big_size = 28;
    let clock_small_size = 13;
    let clock_meta_size = 10;
    let clock_font_big = "";
    let clock_font_small = "";
    let clock_font_meta = "";
    let clock_hm_width = 0;
    const computed_styles = getComputedStyle(document.documentElement);
    const hero_ink = computed_styles.getPropertyValue("--primary").trim() || DEFAULT_HERO_INK;
    const clock_ink = computed_styles.getPropertyValue("--text-strong").trim() || DEFAULT_CLOCK_INK;

    let clock_hh = "";
    let clock_mm = "";
    let clock_ss = "";
    let clock_timer = 0;

    function tick_clock() {
      const now = new Date();
      clock_hh = pad2(now.getHours());
      clock_mm = pad2(now.getMinutes());
      clock_ss = pad2(now.getSeconds());
      if (clock_font_big) {
        hero_ctx.font = clock_font_big;
        clock_hm_width = hero_ctx.measureText(`${clock_hh}:${clock_mm}`).width;
      }
    }

    tick_clock();
    clock_timer = window.setInterval(tick_clock, 1000);

    function resize(next_width: number, next_height: number) {
      width = Math.max(next_width, 280);
      height = Math.max(next_height, 80);
      hero_canvas.width = Math.floor(width * dpr);
      hero_canvas.height = Math.floor(height * dpr);
      hero_canvas.style.width = `${width}px`;
      hero_canvas.style.height = `${height}px`;
      hero_ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

      // 时钟排版在 resize 时一次性计算，避免每一帧重复做字体和尺寸推导。
      clock_pad_x = is_mobile ? 14 : 22;
      clock_pad_y = is_mobile ? 12 : 18;
      clock_big_size = Math.round(Math.min(width * 0.072, height * 0.20, 56));
      clock_small_size = Math.round(clock_big_size * 0.46);
      clock_meta_size = Math.round(Math.min(width * 0.018, 11));
      clock_font_big = `200 ${clock_big_size}px "IBM Plex Mono", monospace`;
      clock_font_small = `200 ${clock_small_size}px "IBM Plex Mono", monospace`;
      clock_font_meta = `400 ${clock_meta_size}px "IBM Plex Mono", monospace`;
      hero_ctx.font = clock_font_big;
      clock_hm_width = hero_ctx.measureText(`${clock_hh}:${clock_mm}`).width;
    }

    function set_pointer(client_x: number, client_y: number) {
      const bounds = hero_canvas.getBoundingClientRect();
      pointer_x = client_x - bounds.left;
      pointer_y = client_y - bounds.top;
    }

    const clear_pointer = () => {
      pointer_x = -9999;
      pointer_y = -9999;
    };

    const on_mouse = (event: MouseEvent) => set_pointer(event.clientX, event.clientY);
    const on_touch = (event: TouchEvent) => {
      const touch = event.touches[0];
      if (touch) {
        set_pointer(touch.clientX, touch.clientY);
      }
    };

    const init = async () => {
      if (frame_id !== 0) {
        cancelAnimationFrame(frame_id);
        frame_id = 0;
      }

      is_mobile = mobile_q.matches;
      const charset = is_mobile ? MOBILE_ASCII_CHARS : ASCII_CHARS;
      const step = is_mobile ? 2 : 4;
      glyph_size = is_mobile ? 3 : 6;
      glyph_font = `500 ${glyph_size}px "IBM Plex Mono", monospace`;
      influence_radius = is_mobile ? 50 : 110;
      influence_radius_sq = influence_radius * influence_radius;
      influence_force = is_mobile ? 5 : 3.5;

      resize(section.clientWidth, section.clientHeight);

      if ("fonts" in document) {
        try {
          await document.fonts.ready;
        } catch {
          // 字体系统失败时退回默认 monospace，动画仍然可以正常工作。
        }
      }

      const metrics_ctx = document.createElement("canvas").getContext("2d");
      if (!metrics_ctx) {
        return;
      }
      metrics_ctx.font = '600 80px "IBM Plex Mono", monospace';
      const measured_width = metrics_ctx.measureText(HERO_LABEL).width || width;

      const fitted_size_by_width = Math.floor((80 * width) / measured_width * 0.92);
      const fitted_size_by_height = Math.floor(height * 0.58);
      const font_size = Math.min(fitted_size_by_width, fitted_size_by_height);
      const hero_font = `600 ${font_size}px "IBM Plex Mono", monospace`;

      const offscreen = document.createElement("canvas");
      offscreen.width = width;
      offscreen.height = height;
      const offscreen_ctx = offscreen.getContext("2d");
      if (!offscreen_ctx) {
        return;
      }
      offscreen_ctx.font = hero_font;
      const text_width = offscreen_ctx.measureText(HERO_LABEL).width;
      offscreen_ctx.fillStyle = "#fff";
      offscreen_ctx.textBaseline = "middle";
      offscreen_ctx.fillText(HERO_LABEL, Math.max(0, (width - text_width) / 2), height * 0.46);

      const image_data = offscreen_ctx.getImageData(0, 0, width, height);
      const next_particles: AsciiParticle[] = [];

      for (let y = 0; y < height; y += step) {
        for (let x = 0; x < width; x += step) {
          if (image_data.data[(y * width + x) * 4 + 3] <= 80) {
            continue;
          }
          next_particles.push({
            x: x + (Math.random() - 0.5) * width * 0.45,
            y: y + (Math.random() - 0.5) * height * 2.2,
            tx: x,
            ty: y,
            vx: 0,
            vy: 0,
            char: pick(charset),
            alpha: 0,
            target_alpha: is_mobile ? 0.95 : 0.82 + Math.random() * 0.18,
            is_text: true,
            phase: Math.random() * Math.PI * 2,
            delay: (x / width) + Math.random() * 0.15,
          });
        }
      }

      const ambient_count = Math.max(40, Math.floor(next_particles.length * 0.12));
      for (let i = 0; i < ambient_count; i += 1) {
        const x = Math.random() * width;
        const y = Math.random() * height;
        next_particles.push({
          x,
          y,
          tx: x,
          ty: y,
          vx: (Math.random() - 0.5) * 0.12,
          vy: (Math.random() - 0.5) * 0.12,
          char: pick(charset),
          alpha: 0,
          target_alpha: 0.03 + Math.random() * 0.06,
          is_text: false,
          phase: Math.random() * Math.PI * 2,
          delay: Math.random() * 0.5,
        });
      }

      particles = next_particles;
      const start_time = performance.now();

      const has_pointer = () => pointer_x > -9000;

      const draw = (now: number) => {
        if (is_dead) {
          frame_id = 0;
          return;
        }

        const elapsed = (now - start_time) / 1000;
        const pointer_active = has_pointer();
        hero_ctx.clearRect(0, 0, width, height);

        hero_ctx.font = glyph_font;
        hero_ctx.textAlign = "center";
        hero_ctx.textBaseline = "middle";
        hero_ctx.fillStyle = hero_ink;

        let last_alpha = -1;

        for (const particle of particles) {
          const progress = Math.max(0, elapsed - particle.delay);

          if (particle.is_text && progress < 0.01) {
            if (last_alpha !== 0.02) {
              hero_ctx.globalAlpha = 0.02;
              last_alpha = 0.02;
            }
            hero_ctx.fillText(particle.char, particle.x, particle.y);
            continue;
          }

          particle.vx += (particle.tx - particle.x) * 0.038;
          particle.vy += (particle.ty - particle.y) * 0.038;

          if (pointer_active) {
            const dx = particle.x - pointer_x;
            const dy = particle.y - pointer_y;
            const distance_sq = dx * dx + dy * dy;
            if (distance_sq < influence_radius_sq && distance_sq > 0) {
              const distance = Math.sqrt(distance_sq);
              const force = ((1 - distance / influence_radius) ** 2) * influence_force;
              particle.vx += (dx / distance) * force;
              particle.vy += (dy / distance) * force;
            }
          }

          particle.vx *= 0.87;
          particle.vy *= 0.87;
          particle.x += particle.vx;
          particle.y += particle.vy;
          particle.alpha += (particle.target_alpha - particle.alpha) * 0.04;

          if (particle.is_text) {
            particle.alpha = particle.target_alpha + Math.sin(elapsed * 0.7 + particle.phase) * 0.07;
            if (progress < 0.9 || Math.random() < 0.0006) {
              particle.char = pick(charset);
            }
          } else {
            particle.tx += (Math.random() - 0.5) * 0.18;
            particle.ty += (Math.random() - 0.5) * 0.18;
            if (particle.x < -20) {
              particle.x = particle.tx = width + 10;
            }
            if (particle.x > width + 20) {
              particle.x = particle.tx = -10;
            }
            if (particle.y < -20) {
              particle.y = particle.ty = height + 10;
            }
            if (particle.y > height + 20) {
              particle.y = particle.ty = -10;
            }
            if (Math.random() < 0.003) {
              particle.char = pick(charset);
            }
          }

          const alpha = Math.max(0, particle.alpha);
          if (alpha !== last_alpha) {
            hero_ctx.globalAlpha = alpha;
            last_alpha = alpha;
          }
          hero_ctx.fillText(particle.char, particle.x, particle.y);
        }

        // 时钟与统计直接画在同一块 canvas 上，避免额外 DOM 叠层。
        hero_ctx.textAlign = "left";
        hero_ctx.textBaseline = "bottom";
        hero_ctx.fillStyle = clock_ink;

        const clock_y = height - clock_pad_y - clock_big_size * 0.28;

        hero_ctx.font = clock_font_big;
        hero_ctx.globalAlpha = 0.82;
        last_alpha = 0.82;
        hero_ctx.fillText(`${clock_hh}:${clock_mm}`, clock_pad_x, clock_y);

        hero_ctx.font = clock_font_small;
        hero_ctx.globalAlpha = 0.38;
        last_alpha = 0.38;
        hero_ctx.fillText(`:${clock_ss}`, clock_pad_x + clock_hm_width + 2, clock_y + (clock_big_size - clock_small_size) * 0.82);

        hero_ctx.font = clock_font_meta;
        hero_ctx.globalAlpha = 0.28;
        hero_ctx.fillText(
          `AGENTS ${agent_count}  ·  ROOMS ${room_count}`,
          clock_pad_x,
          height - clock_pad_y - clock_big_size - 6,
        );

        hero_ctx.globalAlpha = 1;
        frame_id = requestAnimationFrame(draw);
      };

      frame_id = requestAnimationFrame(draw);
    };

    const resize_observer = new ResizeObserver(() => {
      void init();
    });
    resize_observer.observe(section);

    hero_canvas.addEventListener("mousemove", on_mouse, { passive: true });
    hero_canvas.addEventListener("mouseleave", clear_pointer);
    hero_canvas.addEventListener("touchstart", on_touch, { passive: true });
    hero_canvas.addEventListener("touchmove", on_touch, { passive: true });
    hero_canvas.addEventListener("touchend", clear_pointer);

    void init();

    return () => {
      is_dead = true;
      clearInterval(clock_timer);
      if (frame_id !== 0) {
        cancelAnimationFrame(frame_id);
      }
      resize_observer.disconnect();
      hero_canvas.removeEventListener("mousemove", on_mouse);
      hero_canvas.removeEventListener("mouseleave", clear_pointer);
      hero_canvas.removeEventListener("touchstart", on_touch);
      hero_canvas.removeEventListener("touchmove", on_touch);
      hero_canvas.removeEventListener("touchend", clear_pointer);
    };
  }, [prefers_reduced_motion, agent_count, room_count, theme]);

  return (
    <div
      ref={section_ref}
      className="relative h-full w-full overflow-hidden rounded-[28px] border"
      style={{
        background: "var(--surface-canvas-background)",
        borderColor: "var(--surface-canvas-border)",
      }}
    >
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background: "radial-gradient(ellipse 60% 50% at 50% 50%, color-mix(in srgb, var(--primary) 12%, transparent), transparent)",
        }}
      />

      <h2 className="sr-only">{HERO_LABEL}</h2>

      {prefers_reduced_motion ? (
        <div
          className="absolute inset-0 flex items-center justify-center font-mono text-[clamp(3rem,11vw,6.8rem)] font-light italic leading-none"
          style={{ color: "var(--primary)" }}
        >
          {HERO_LABEL}
        </div>
      ) : (
        <canvas
          ref={canvas_ref}
          className="absolute inset-0 block cursor-crosshair"
        />
      )}
    </div>
  );
}
