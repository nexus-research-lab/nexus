"use client";

import { useEffect, useRef } from "react";

import { useTheme } from "./theme-context";

/* ─────────────────────────────────────
   Rain canvas — 三层视差雨滴 + 水花溅射
   性能关键点：
   - DPR 缩放 → Retina 清晰渲染
   - 按 lineWidth 分组 stroke → 减少状态切换
   - active ref 控制 start/stop → 非激活不跑空帧
   ───────────────────────────────────── */

interface RainDrop {
  x: number; y: number;
  len: number; speed: number; w: number;
  r: number; g: number; b: number; alpha: number;
  drift: number; splash_chance: number;
}
interface Splash {
  x: number; y: number;
  radius: number; max_radius: number;
  alpha: number; life: number; max_life: number;
}

const RAIN_LAYERS = [
  { count: 70, len: [12, 22] as [number, number], speed: 20, w: 1.0, c: [200, 210, 225] as const, a: [0.20, 0.38] as [number, number], sc: 0.30 },
  { count: 150, len: [7, 15] as [number, number], speed: 14, w: 0.7, c: [185, 195, 212] as const, a: [0.12, 0.25] as [number, number], sc: 0.05 },
  { count: 120, len: [4, 9] as [number, number], speed: 9, w: 0.4, c: [170, 180, 200] as const, a: [0.06, 0.15] as [number, number], sc: 0.02 },
];

function make_drops(W: number, H: number): RainDrop[] {
  return RAIN_LAYERS.flatMap(l =>
    Array.from({ length: l.count }, () => ({
      x: Math.random() * (W + 60) - 30,
      y: Math.random() * H,
      len: l.len[0] + Math.random() * (l.len[1] - l.len[0]),
      speed: l.speed + Math.random() * l.speed * 0.4,
      w: l.w + Math.random() * 0.15,
      r: l.c[0], g: l.c[1], b: l.c[2],
      alpha: l.a[0] + Math.random() * (l.a[1] - l.a[0]),
      drift: 1.5 + Math.random(),
      splash_chance: l.sc,
    }))
  );
}

/** 雨滴 + 水花渲染（纯函数，无副作用） */
function draw_rain(
  ctx: CanvasRenderingContext2D,
  W: number, H: number,
  drops: RainDrop[],
  splashes: Splash[],
) {
  ctx.clearRect(0, 0, W, H);

  let offset = 0;
  for (const layer of RAIN_LAYERS) {
    ctx.lineWidth = layer.w;
    ctx.lineCap = "round";
    for (let i = offset, end = offset + layer.count; i < end; i++) {
      const d = drops[i];
      ctx.beginPath();
      ctx.moveTo(d.x, d.y);
      ctx.lineTo(d.x + d.drift * (d.len / d.speed), d.y + d.len);
      ctx.strokeStyle = `rgba(${d.r},${d.g},${d.b},${d.alpha})`;
      ctx.stroke();

      d.y += d.speed;
      d.x += d.drift;
      if (d.y > H) {
        if (Math.random() < d.splash_chance)
          splashes.push({
            x: d.x, y: H - 2 + Math.random() * 4,
            radius: 0, max_radius: 2 + Math.random() * 3,
            alpha: 0.2 + Math.random() * 0.15,
            life: 0, max_life: 8 + Math.random() * 6,
          });
        d.y = -d.len - Math.random() * 80;
        d.x = Math.random() * (W + 60) - 30;
      }
    }
    offset += layer.count;
  }

  ctx.lineWidth = 0.5;
  for (let i = splashes.length - 1; i >= 0; i--) {
    const s = splashes[i];
    s.life++;
    s.radius = s.max_radius * (s.life / s.max_life);
    const a = s.alpha * (1 - s.life / s.max_life);
    if (a <= 0 || s.life >= s.max_life) { splashes.splice(i, 1); continue; }
    ctx.beginPath();
    ctx.ellipse(s.x, s.y, s.radius * 1.5, s.radius * 0.5, 0, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(180,190,205,${a})`;
    ctx.stroke();
  }
}

function RainCanvas({ active }: { active: boolean }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const stateRef = useRef({
    raf: 0,
    W: 0, H: 0,
    drops: [] as RainDrop[],
    splashes: [] as Splash[],
  });

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d")!;
    const s = stateRef.current;

    /* ── resize：DPR 缩放 + 重新生成雨滴 ── */
    const resize = () => {
      const dpr = window.devicePixelRatio || 1;
      s.W = window.innerWidth;
      s.H = window.innerHeight;
      canvas.width = s.W * dpr;
      canvas.height = s.H * dpr;
      canvas.style.width = s.W + "px";
      canvas.style.height = s.H + "px";
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      s.drops = make_drops(s.W, s.H);
      s.splashes.length = 0;
    };

    resize();
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, []);

  /* ── active 变化 → 启动/停止 RAF ── */
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d")!;
    const s = stateRef.current;

    if (!active) {
      cancelAnimationFrame(s.raf);
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      return;
    }

    /* 确保 DPR transform 正确（从其他主题切回时 resize 可能没触发） */
    const dpr = window.devicePixelRatio || 1;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    let running = true;
    const tick = () => {
      if (!running) return;
      draw_rain(ctx, s.W, s.H, s.drops, s.splashes);
      s.raf = requestAnimationFrame(tick);
    };
    s.raf = requestAnimationFrame(tick);

    return () => {
      running = false;
      cancelAnimationFrame(s.raf);
    };
  }, [active]);

  return <canvas ref={canvasRef} style={{ position: "fixed", inset: 0, pointerEvents: "none" }} />;
}

function SunnyLeavesVideo({ active }: { active: boolean }) {
  const video_ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = video_ref.current;
    if (!video) {
      return;
    }

    if (!active) {
      video.pause();
      video.currentTime = 0;
      return;
    }

    const play_result = video.play();
    if (play_result && typeof play_result.catch === "function") {
      play_result.catch(() => { });
    }
  }, [active]);

  return (
    <video
      ref={video_ref}
      src="/sunny/leaves.mp4"
      muted
      loop
      playsInline
      preload="metadata"
      style={{
        width: "100%",
        height: "100%",
        objectFit: "cover",
        objectPosition: "center 58%",
        mixBlendMode: "multiply",
      }}
    />
  );
}

/* ─────────────────────────────────────
   ThemeOverlay — 根组件
   ───────────────────────────────────── */

export function ThemeOverlay() {
  const { theme } = useTheme();
  const T = "opacity 700ms var(--motion-ease-standard)";
  const is_sunny = theme === "sunny";
  const is_rain = theme === "rain";

  return (
    <>
      {/* ── Sunny leaves overlay：亮色底盘复用 light，只叠加轻量树荫视频层 ── */}
      {is_sunny ? (
        <div
          aria-hidden
          style={{
            position: "fixed", inset: 0, pointerEvents: "none", zIndex: 995,
            opacity: 0.38, transition: T,
            WebkitMaskImage: "linear-gradient(180deg, rgba(0,0,0,0.96) 0%, rgba(0,0,0,0.9) 18%, rgba(0,0,0,0.66) 42%, rgba(0,0,0,0.28) 68%, rgba(0,0,0,0.08) 82%, transparent 92%)",
            maskImage: "linear-gradient(180deg, rgba(0,0,0,0.96) 0%, rgba(0,0,0,0.9) 18%, rgba(0,0,0,0.66) 42%, rgba(0,0,0,0.28) 68%, rgba(0,0,0,0.08) 82%, transparent 92%)",
          }}
        >
          <SunnyLeavesVideo active={is_sunny} />
        </div>
      ) : null}

      {/* ── Rain fog layer ── */}
      {is_rain ? (
        <div
          aria-hidden
          style={{
            position: "fixed", inset: 0, pointerEvents: "none", zIndex: 996,
            opacity: 1, transition: T,
            background: "radial-gradient(ellipse at 50% 100%,rgba(70,80,95,0.18) 0%,transparent 45%),radial-gradient(ellipse at 20% 85%,rgba(60,70,85,0.1) 0%,transparent 35%)",
            animation: "nexus-fog-drift 25s ease-in-out infinite alternate",
          }}
        />
      ) : null}

      {/* ── Rain canvas ── */}
      {is_rain ? (
        <div
          aria-hidden
          style={{
            position: "fixed", inset: 0, pointerEvents: "none", zIndex: 997,
            opacity: 1, transition: T,
          }}
        >
          <RainCanvas active={is_rain} />
        </div>
      ) : null}
    </>
  );
}
