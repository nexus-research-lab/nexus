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

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { useTheme } from "@/shared/theme/theme-context";

const ASCII_CHARS = ".:+-=*#@&~<>{}[]|/\\";
const MOBILE_ASCII_CHARS = "01";
const HERO_LABEL = "nexus";
const DEFAULT_HERO_INK = "#516dff";
const DEFAULT_CLOCK_INK = "rgba(32, 45, 62, 0.88)";

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

function shouldReduceHomeHeroMotion(prefersReducedMotion: boolean) {
  if (!prefersReducedMotion) {
    return false;
  }

  const runtimeConfig = getDesktopRuntimeConfig();
  return runtimeConfig?.appMode !== "desktop" || runtimeConfig.platform !== "windows";
}

export function HomeAsciiHero() {
  const { theme } = useTheme();
  const sectionRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const prefersReducedMotion = usePrefersReducedMotion();
  const shouldReduceMotion = shouldReduceHomeHeroMotion(prefersReducedMotion);

  useEffect(() => {
    const section = sectionRef.current;
    const canvas = canvasRef.current;
    if (!section || !canvas || shouldReduceMotion) {
      return;
    }

    const ctx = canvas.getContext("2d");
    if (!ctx) {
      return;
    }
    const heroCanvas = canvas;
    const heroCtx = ctx;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const mobileQ = window.matchMedia("(max-width: 600px)");

    let particles: AsciiParticle[] = [];
    let width = 0;
    let height = 0;
    let glyphSize = 6;
    let glyphFont = "";
    let influenceRadius = 100;
    let influenceRadiusSq = influenceRadius * influenceRadius;
    let influenceForce = 3;
    let frameId = 0;
    let pointerX = -9999;
    let pointerY = -9999;
    let isDead = false;
    let isMobile = false;
    let clockPadX = 22;
    let clockPadY = 18;
    let clockBigSize = 28;
    let clockSmallSize = 13;
    let clockFontBig = "";
    let clockFontSmall = "";
    let clockHmWidth = 0;
    const computedStyles = getComputedStyle(document.documentElement);
    const heroInk = computedStyles.getPropertyValue("--primary").trim() || DEFAULT_HERO_INK;
    const clockInk = computedStyles.getPropertyValue("--text-strong").trim() || DEFAULT_CLOCK_INK;

    let clockHh = "";
    let clockMm = "";
    let clockSs = "";
    let clockTimer = 0;

    function tickClock() {
      const now = new Date();
      clockHh = pad2(now.getHours());
      clockMm = pad2(now.getMinutes());
      clockSs = pad2(now.getSeconds());
      if (clockFontBig) {
        heroCtx.font = clockFontBig;
        clockHmWidth = heroCtx.measureText(`${clockHh}:${clockMm}`).width;
      }
    }

    tickClock();
    clockTimer = window.setInterval(tickClock, 1000);

    function resize(nextWidth: number, nextHeight: number) {
      width = Math.max(nextWidth, 280);
      height = Math.max(nextHeight, 80);
      heroCanvas.width = Math.floor(width * dpr);
      heroCanvas.height = Math.floor(height * dpr);
      heroCanvas.style.width = `${width}px`;
      heroCanvas.style.height = `${height}px`;
      heroCtx.setTransform(dpr, 0, 0, dpr, 0, 0);

      // 时钟排版在 resize 时一次性计算，避免每一帧重复做字体和尺寸推导。
      clockPadX = isMobile ? 14 : 22;
      clockPadY = isMobile ? 12 : 18;
      clockBigSize = Math.round(Math.min(width * 0.072, height * 0.20, 56));
      clockSmallSize = Math.round(clockBigSize * 0.46);
      clockFontBig = `200 ${clockBigSize}px "IBM Plex Mono", monospace`;
      clockFontSmall = `200 ${clockSmallSize}px "IBM Plex Mono", monospace`;
      heroCtx.font = clockFontBig;
      clockHmWidth = heroCtx.measureText(`${clockHh}:${clockMm}`).width;
    }

    function setPointer(clientX: number, clientY: number) {
      const bounds = heroCanvas.getBoundingClientRect();
      pointerX = clientX - bounds.left;
      pointerY = clientY - bounds.top;
    }

    const clearPointer = () => {
      pointerX = -9999;
      pointerY = -9999;
    };

    const onMouse = (event: MouseEvent) => setPointer(event.clientX, event.clientY);
    const onTouch = (event: TouchEvent) => {
      const touch = event.touches[0];
      if (touch) {
        setPointer(touch.clientX, touch.clientY);
      }
    };

    const init = async () => {
      if (frameId !== 0) {
        cancelAnimationFrame(frameId);
        frameId = 0;
      }

      isMobile = mobileQ.matches;
      const charset = isMobile ? MOBILE_ASCII_CHARS : ASCII_CHARS;
      const step = isMobile ? 2 : 4;
      glyphSize = isMobile ? 3 : 6;
      glyphFont = `500 ${glyphSize}px "IBM Plex Mono", monospace`;
      influenceRadius = isMobile ? 50 : 110;
      influenceRadiusSq = influenceRadius * influenceRadius;
      influenceForce = isMobile ? 5 : 3.5;

      resize(section.clientWidth, section.clientHeight);

      if ("fonts" in document) {
        try {
          await document.fonts.ready;
        } catch {
          // 字体系统失败时退回默认 monospace，动画仍然可以正常工作。
        }
      }

      const metricsCtx = document.createElement("canvas").getContext("2d");
      if (!metricsCtx) {
        return;
      }
      metricsCtx.font = '600 80px "IBM Plex Mono", monospace';
      const measuredWidth = metricsCtx.measureText(HERO_LABEL).width || width;

      const fittedSizeByWidth = Math.floor((80 * width) / measuredWidth * 0.92);
      const fittedSizeByHeight = Math.floor(height * 0.58);
      const fontSize = Math.min(fittedSizeByWidth, fittedSizeByHeight);
      const heroFont = `600 ${fontSize}px "IBM Plex Mono", monospace`;

      const offscreen = document.createElement("canvas");
      offscreen.width = width;
      offscreen.height = height;
      const offscreenCtx = offscreen.getContext("2d");
      if (!offscreenCtx) {
        return;
      }
      offscreenCtx.font = heroFont;
      const textWidth = offscreenCtx.measureText(HERO_LABEL).width;
      offscreenCtx.fillStyle = "#fff";
      offscreenCtx.textBaseline = "middle";
      offscreenCtx.fillText(HERO_LABEL, Math.max(0, (width - textWidth) / 2), height * 0.46);

      const imageData = offscreenCtx.getImageData(0, 0, width, height);
      const nextParticles: AsciiParticle[] = [];

      for (let y = 0; y < height; y += step) {
        for (let x = 0; x < width; x += step) {
          if (imageData.data[(y * width + x) * 4 + 3] <= 80) {
            continue;
          }
          nextParticles.push({
            x: x + (Math.random() - 0.5) * width * 0.45,
            y: y + (Math.random() - 0.5) * height * 2.2,
            tx: x,
            ty: y,
            vx: 0,
            vy: 0,
            char: pick(charset),
            alpha: 0,
            target_alpha: isMobile ? 0.95 : 0.82 + Math.random() * 0.18,
            is_text: true,
            phase: Math.random() * Math.PI * 2,
            delay: (x / width) + Math.random() * 0.15,
          });
        }
      }

      const ambientCount = Math.max(40, Math.floor(nextParticles.length * 0.12));
      for (let i = 0; i < ambientCount; i += 1) {
        const x = Math.random() * width;
        const y = Math.random() * height;
        nextParticles.push({
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

      particles = nextParticles;
      const startTime = performance.now();

      const hasPointer = () => pointerX > -9000;

      const draw = (now: number) => {
        if (isDead) {
          frameId = 0;
          return;
        }

        const elapsed = (now - startTime) / 1000;
        const pointerActive = hasPointer();
        heroCtx.clearRect(0, 0, width, height);

        heroCtx.font = glyphFont;
        heroCtx.textAlign = "center";
        heroCtx.textBaseline = "middle";
        heroCtx.fillStyle = heroInk;

        let lastAlpha = -1;

        for (const particle of particles) {
          const progress = Math.max(0, elapsed - particle.delay);

          if (particle.is_text && progress < 0.01) {
            if (lastAlpha !== 0.02) {
              heroCtx.globalAlpha = 0.02;
              lastAlpha = 0.02;
            }
            heroCtx.fillText(particle.char, particle.x, particle.y);
            continue;
          }

          particle.vx += (particle.tx - particle.x) * 0.038;
          particle.vy += (particle.ty - particle.y) * 0.038;

          if (pointerActive) {
            const dx = particle.x - pointerX;
            const dy = particle.y - pointerY;
            const distanceSq = dx * dx + dy * dy;
            if (distanceSq < influenceRadiusSq && distanceSq > 0) {
              const distance = Math.sqrt(distanceSq);
              const force = ((1 - distance / influenceRadius) ** 2) * influenceForce;
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
          if (alpha !== lastAlpha) {
            heroCtx.globalAlpha = alpha;
            lastAlpha = alpha;
          }
          heroCtx.fillText(particle.char, particle.x, particle.y);
        }

        // 时钟与统计直接画在同一块 canvas 上，避免额外 DOM 叠层。
        heroCtx.textAlign = "left";
        heroCtx.textBaseline = "bottom";
        heroCtx.fillStyle = clockInk;

        const clockY = height - clockPadY - clockBigSize * 0.28;

        heroCtx.font = clockFontBig;
        heroCtx.globalAlpha = 0.82;
        lastAlpha = 0.82;
        heroCtx.fillText(`${clockHh}:${clockMm}`, clockPadX, clockY);

        heroCtx.font = clockFontSmall;
        heroCtx.globalAlpha = 0.38;
        lastAlpha = 0.38;
        heroCtx.fillText(`:${clockSs}`, clockPadX + clockHmWidth + 2, clockY + (clockBigSize - clockSmallSize) * 0.82);

        heroCtx.globalAlpha = 1;
        frameId = requestAnimationFrame(draw);
      };

      frameId = requestAnimationFrame(draw);
    };

    const resizeObserver = new ResizeObserver(() => {
      void init();
    });
    resizeObserver.observe(section);

    heroCanvas.addEventListener("mousemove", onMouse, { passive: true });
    heroCanvas.addEventListener("mouseleave", clearPointer);
    heroCanvas.addEventListener("touchstart", onTouch, { passive: true });
    heroCanvas.addEventListener("touchmove", onTouch, { passive: true });
    heroCanvas.addEventListener("touchend", clearPointer);

    void init();

    return () => {
      isDead = true;
      clearInterval(clockTimer);
      if (frameId !== 0) {
        cancelAnimationFrame(frameId);
      }
      resizeObserver.disconnect();
      heroCanvas.removeEventListener("mousemove", onMouse);
      heroCanvas.removeEventListener("mouseleave", clearPointer);
      heroCanvas.removeEventListener("touchstart", onTouch);
      heroCanvas.removeEventListener("touchmove", onTouch);
      heroCanvas.removeEventListener("touchend", clearPointer);
    };
  }, [shouldReduceMotion, theme]);

  return (
    <div
      ref={sectionRef}
      className="relative h-full w-full overflow-hidden rounded-[14px] border"
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

      {shouldReduceMotion ? (
        <div
          className="absolute inset-0 flex items-center justify-center font-mono text-[clamp(3rem,11vw,6.8rem)] font-light italic leading-none"
          style={{ color: "var(--primary)" }}
        >
          {HERO_LABEL}
        </div>
      ) : (
        <canvas
          ref={canvasRef}
          className="absolute inset-0 block cursor-crosshair"
        />
      )}
    </div>
  );
}
