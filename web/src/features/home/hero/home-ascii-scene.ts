import {
  createHomeAsciiParticleField,
  type HomeAsciiParticle,
  type HomeAsciiPointer,
  updateHomeAsciiParticle,
} from "./home-ascii-particle-model";

export const HOME_HERO_LABEL = "nexus";

const DESKTOP_CHARSET = ".:+-=*#@&~<>{}[]|/\\";
const MOBILE_CHARSET = "01";
const DEFAULT_HERO_INK = "#516dff";
const DEFAULT_CLOCK_INK = "rgba(32, 45, 62, 0.88)";

interface HomeAsciiPalette {
  clockInk: string;
  heroInk: string;
}

interface HomeAsciiViewport {
  charset: string;
  dpr: number;
  glyphFont: string;
  height: number;
  influenceForce: number;
  influenceRadius: number;
  isMobile: boolean;
  step: number;
  width: number;
}

interface HomeAsciiClock {
  bigSize: number;
  fontBig: string;
  fontSmall: string;
  hh: string;
  hmWidth: number;
  mm: string;
  padX: number;
  padY: number;
  smallSize: number;
  ss: string;
}

const INITIAL_VIEWPORT: HomeAsciiViewport = {
  charset: DESKTOP_CHARSET,
  dpr: 1,
  glyphFont: '500 6px "IBM Plex Mono", monospace',
  height: 80,
  influenceForce: 3.5,
  influenceRadius: 110,
  isMobile: false,
  step: 4,
  width: 280,
};

const INITIAL_CLOCK: HomeAsciiClock = {
  bigSize: 28,
  fontBig: "",
  fontSmall: "",
  hh: "00",
  hmWidth: 0,
  mm: "00",
  padX: 22,
  padY: 18,
  smallSize: 13,
  ss: "00",
};

export function createHomeAsciiScene(
  section: HTMLElement,
  canvas: HTMLCanvasElement,
): HomeAsciiScene | null {
  const context = canvas.getContext("2d");
  if (!context) {
    return null;
  }
  const styles = getComputedStyle(document.documentElement);
  return new HomeAsciiScene(section, canvas, context, {
    clockInk: styles.getPropertyValue("--text-strong").trim() || DEFAULT_CLOCK_INK,
    heroInk: styles.getPropertyValue("--primary").trim() || DEFAULT_HERO_INK,
  });
}

export class HomeAsciiScene {
  private readonly canvas: HTMLCanvasElement;
  private readonly context: CanvasRenderingContext2D;
  private readonly mobileMedia = window.matchMedia("(max-width: 600px)");
  private readonly palette: HomeAsciiPalette;
  private readonly section: HTMLElement;
  private clock: HomeAsciiClock = { ...INITIAL_CLOCK };
  private clockTimer = 0;
  private disposed = false;
  private frameId = 0;
  private animationStart = 0;
  private particles: HomeAsciiParticle[] = [];
  private pointer: HomeAsciiPointer | null = null;
  private rebuildFrameId = 0;
  private rebuildVersion = 0;
  private resizeObserver: ResizeObserver | null = null;
  private viewport: HomeAsciiViewport = { ...INITIAL_VIEWPORT };

  constructor(
    section: HTMLElement,
    canvas: HTMLCanvasElement,
    context: CanvasRenderingContext2D,
    palette: HomeAsciiPalette,
  ) {
    this.section = section;
    this.canvas = canvas;
    this.context = context;
    this.palette = palette;
  }

  start(): void {
    this.tickClock();
    this.clockTimer = window.setInterval(this.tickClock, 1000);
    this.resizeObserver = new ResizeObserver(this.scheduleRebuild);
    this.resizeObserver.observe(this.section);
    this.canvas.addEventListener("mousemove", this.handleMouseMove, { passive: true });
    this.canvas.addEventListener("mouseleave", this.clearPointer);
    this.canvas.addEventListener("touchstart", this.handleTouch, { passive: true });
    this.canvas.addEventListener("touchmove", this.handleTouch, { passive: true });
    this.canvas.addEventListener("touchend", this.clearPointer);
    void this.rebuild();
  }

  dispose(): void {
    this.disposed = true;
    this.rebuildVersion += 1;
    window.clearInterval(this.clockTimer);
    this.stopAnimation();
    if (this.rebuildFrameId !== 0) {
      window.cancelAnimationFrame(this.rebuildFrameId);
    }
    this.resizeObserver?.disconnect();
    this.canvas.removeEventListener("mousemove", this.handleMouseMove);
    this.canvas.removeEventListener("mouseleave", this.clearPointer);
    this.canvas.removeEventListener("touchstart", this.handleTouch);
    this.canvas.removeEventListener("touchmove", this.handleTouch);
    this.canvas.removeEventListener("touchend", this.clearPointer);
  }

  private readonly scheduleRebuild = () => {
    if (this.rebuildFrameId !== 0 || this.disposed) {
      return;
    }
    this.rebuildFrameId = window.requestAnimationFrame(() => {
      this.rebuildFrameId = 0;
      void this.rebuild();
    });
  };

  private async rebuild(): Promise<void> {
    const version = ++this.rebuildVersion;
    this.stopAnimation();
    this.configureViewport();
    this.resizeCanvas();
    await this.waitForFonts();
    if (this.disposed || version !== this.rebuildVersion) {
      return;
    }

    const imageData = this.createTextMask();
    if (!imageData) {
      return;
    }
    this.particles = createHomeAsciiParticleField({
      charset: this.viewport.charset,
      height: this.viewport.height,
      imageData,
      isMobile: this.viewport.isMobile,
      step: this.viewport.step,
      width: this.viewport.width,
    });
    this.animationStart = performance.now();
    this.frameId = window.requestAnimationFrame(this.drawFrame);
  }

  private configureViewport(): void {
    const isMobile = this.mobileMedia.matches;
    const glyphSize = isMobile ? 3 : 6;
    this.viewport = {
      charset: isMobile ? MOBILE_CHARSET : DESKTOP_CHARSET,
      dpr: Math.min(window.devicePixelRatio || 1, 2),
      glyphFont: `500 ${glyphSize}px "IBM Plex Mono", monospace`,
      height: Math.round(Math.max(this.section.clientHeight, 80)),
      influenceForce: isMobile ? 5 : 3.5,
      influenceRadius: isMobile ? 50 : 110,
      isMobile,
      step: isMobile ? 2 : 4,
      width: Math.round(Math.max(this.section.clientWidth, 280)),
    };
  }

  private resizeCanvas(): void {
    const { dpr, height, isMobile, width } = this.viewport;
    this.canvas.width = Math.floor(width * dpr);
    this.canvas.height = Math.floor(height * dpr);
    this.canvas.style.width = `${width}px`;
    this.canvas.style.height = `${height}px`;
    this.context.setTransform(dpr, 0, 0, dpr, 0, 0);

    const bigSize = Math.round(Math.min(width * 0.072, height * 0.2, 56));
    const smallSize = Math.round(bigSize * 0.46);
    this.clock = {
      ...this.clock,
      bigSize,
      fontBig: `200 ${bigSize}px "IBM Plex Mono", monospace`,
      fontSmall: `200 ${smallSize}px "IBM Plex Mono", monospace`,
      padX: isMobile ? 14 : 22,
      padY: isMobile ? 12 : 18,
      smallSize,
    };
    this.measureClock();
  }

  private async waitForFonts(): Promise<void> {
    if (!("fonts" in document)) {
      return;
    }
    try {
      await document.fonts.ready;
    } catch {
      // 字体加载失败不阻断场景，Canvas 会使用系统等宽字体。
    }
  }

  private createTextMask(): ImageData | null {
    const { height, width } = this.viewport;
    const metricsContext = document.createElement("canvas").getContext("2d");
    if (!metricsContext) {
      return null;
    }
    metricsContext.font = '600 80px "IBM Plex Mono", monospace';
    const measuredWidth = metricsContext.measureText(HOME_HERO_LABEL).width || width;
    const fontSize = Math.min(
      Math.floor((80 * width) / measuredWidth * 0.92),
      Math.floor(height * 0.58),
    );

    const offscreen = document.createElement("canvas");
    offscreen.width = width;
    offscreen.height = height;
    const context = offscreen.getContext("2d");
    if (!context) {
      return null;
    }
    context.font = `600 ${fontSize}px "IBM Plex Mono", monospace`;
    const textWidth = context.measureText(HOME_HERO_LABEL).width;
    context.fillStyle = "#fff";
    context.textBaseline = "middle";
    context.fillText(
      HOME_HERO_LABEL,
      Math.max(0, (width - textWidth) / 2),
      height * 0.46,
    );
    return context.getImageData(0, 0, width, height);
  }

  private readonly drawFrame = (now: number) => {
    if (this.disposed) {
      this.frameId = 0;
      return;
    }
    const elapsed = (now - this.animationStart) / 1000;
    this.drawParticles(elapsed);
    this.drawClock();
    this.frameId = window.requestAnimationFrame(this.drawFrame);
  };

  private drawParticles(elapsed: number): void {
    const { charset, glyphFont, height, influenceForce, influenceRadius, width } = this.viewport;
    this.context.clearRect(0, 0, width, height);
    this.context.font = glyphFont;
    this.context.textAlign = "center";
    this.context.textBaseline = "middle";
    this.context.fillStyle = this.palette.heroInk;

    let lastAlpha = -1;
    const frame = {
      charset,
      elapsed,
      height,
      influenceForce,
      influenceRadius,
      pointer: this.pointer,
      width,
    };
    for (const particle of this.particles) {
      const alpha = updateHomeAsciiParticle(particle, frame);
      if (alpha !== lastAlpha) {
        this.context.globalAlpha = alpha;
        lastAlpha = alpha;
      }
      this.context.fillText(particle.char, particle.x, particle.y);
    }
  }

  private drawClock(): void {
    const { height } = this.viewport;
    const clockY = height - this.clock.padY - this.clock.bigSize * 0.28;
    this.context.textAlign = "left";
    this.context.textBaseline = "bottom";
    this.context.fillStyle = this.palette.clockInk;
    this.context.font = this.clock.fontBig;
    this.context.globalAlpha = 0.82;
    this.context.fillText(`${this.clock.hh}:${this.clock.mm}`, this.clock.padX, clockY);
    this.context.font = this.clock.fontSmall;
    this.context.globalAlpha = 0.38;
    this.context.fillText(
      `:${this.clock.ss}`,
      this.clock.padX + this.clock.hmWidth + 2,
      clockY + (this.clock.bigSize - this.clock.smallSize) * 0.82,
    );
    this.context.globalAlpha = 1;
  }

  private readonly tickClock = () => {
    const now = new Date();
    this.clock = {
      ...this.clock,
      hh: pad2(now.getHours()),
      mm: pad2(now.getMinutes()),
      ss: pad2(now.getSeconds()),
    };
    this.measureClock();
  };

  private measureClock(): void {
    if (!this.clock.fontBig) {
      return;
    }
    this.context.font = this.clock.fontBig;
    this.clock.hmWidth = this.context.measureText(
      `${this.clock.hh}:${this.clock.mm}`,
    ).width;
  }

  private readonly handleMouseMove = (event: MouseEvent) => {
    this.setPointer(event.clientX, event.clientY);
  };

  private readonly handleTouch = (event: TouchEvent) => {
    const touch = event.touches[0];
    if (touch) {
      this.setPointer(touch.clientX, touch.clientY);
    }
  };

  private setPointer(clientX: number, clientY: number): void {
    const bounds = this.canvas.getBoundingClientRect();
    this.pointer = {
      x: clientX - bounds.left,
      y: clientY - bounds.top,
    };
  }

  private readonly clearPointer = () => {
    this.pointer = null;
  };

  private stopAnimation(): void {
    if (this.frameId === 0) {
      return;
    }
    window.cancelAnimationFrame(this.frameId);
    this.frameId = 0;
  }
}

function pad2(value: number): string {
  return value.toString().padStart(2, "0");
}
