const CURSOR_MOVE_MESSAGE = "NEXUS_CURSOR_MOVE";
const CURSOR_HIDE_MESSAGE = "NEXUS_CURSOR_HIDE";
const CURSOR_MIN_MOVE_DURATION_MS = 240;
const CURSOR_MAX_MOVE_DURATION_MS = 680;
const CURSOR_LONG_MOVE_THRESHOLD = 196;
const CURSOR_SETTLE_DURATION_MS = 1410;
const CURSOR_SETTLE_PERIOD_SECONDS = 0.66;
const CURSOR_SETTLE_ANGLE_DEGREES = 12.5;
const CURSOR_HOTSPOT = { x: 2.5, y: 1.8 };

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(maximum, value));
}

function round(value) {
  return Math.round(value * 1000) / 1000;
}

function smoothstep(start, end, value) {
  const progress = clamp((value - start) / (end - start), 0, 1);
  return progress * progress * (3 - 2 * progress);
}

function springProgress(progress) {
  const response = 6.5;
  const value = 1 - (1 + response * progress) * Math.exp(-response * progress);
  const end = 1 - (1 + response) * Math.exp(-response);
  return clamp(value / end, 0, 1);
}

function normalizeAngle(degrees) {
  let value = degrees % 360;
  if (value > 180) value -= 360;
  if (value < -180) value += 360;
  return value;
}

class NexusCursorOverlay {
  constructor() {
    this.host = null;
    this.cursor = null;
    this.pointer = null;
    this.shape = null;
    this.current = null;
    this.motionAnimation = null;
    this.settleAnimation = null;
    this.entranceAnimation = null;
    this.visible = false;
    this.observer = new MutationObserver(() => {
      if (this.host && !this.host.isConnected) this.mount();
    });
    this.observer.observe(document, { childList: true, subtree: true });
    document.addEventListener("visibilitychange", () => {
      if (document.hidden) this.hide();
    });
  }

  mount() {
    if (this.host?.isConnected || !document.documentElement) return;
    this.cancelAnimations();

    const host = document.createElement("div");
    host.id = "nexus-agent-cursor-root";
    for (const [property, value] of Object.entries({
      all: "initial",
      inset: "0",
      pointerEvents: "none",
      position: "fixed",
      zIndex: "2147483646",
    })) {
      host.style.setProperty(property.replace(/[A-Z]/g, (letter) => "-" + letter.toLowerCase()), value, "important");
    }

    const shadow = host.attachShadow({ mode: "closed" });
    const style = document.createElement("style");
    style.textContent = `
      .cursor {
        filter: drop-shadow(0 1px 1px rgba(15, 23, 42, .42))
          drop-shadow(0 0 6px rgba(117, 103, 237, .82))
          drop-shadow(0 0 15px rgba(117, 103, 237, .42));
        height: 24px;
        left: 0;
        opacity: 0;
        pointer-events: none;
        position: absolute;
        top: 0;
        transform-origin: ${CURSOR_HOTSPOT.x}px ${CURSOR_HOTSPOT.y}px;
        transition: opacity 180ms ease;
        width: 23px;
        will-change: transform, opacity;
      }
      .pointer, .shape {
        height: 100%;
        transform-origin: ${CURSOR_HOTSPOT.x}px ${CURSOR_HOTSPOT.y}px;
        width: 100%;
      }
      .cursor svg { display: block; height: 100%; width: 100%; }
      @media print { .cursor { display: none; } }
      @media (prefers-reduced-motion: reduce) { .cursor { transition: none; } }
    `;
    const cursor = document.createElement("div");
    cursor.className = "cursor";
    cursor.setAttribute("aria-hidden", "true");
    cursor.innerHTML = `
      <div class="pointer">
        <div class="shape">
          <svg viewBox="0 0 23 24" xmlns="http://www.w3.org/2000/svg">
            <path d="M2.45 1.6 20.72 14.64c.93.66.55 2.12-.57 2.32l-6.92 1.25-4.17 6.17c-.66.98-2.15.7-2.48-.46L.92 3.76C.55 2.39 1.38.84 2.45 1.6Z" fill="#64748b" fill-opacity=".82" stroke="#fff" stroke-width="1.8" stroke-linejoin="round"/>
          </svg>
        </div>
      </div>
    `;
    shadow.append(style, cursor);
    document.documentElement.appendChild(host);

    this.host = host;
    this.cursor = cursor;
    this.pointer = cursor.querySelector(".pointer");
    this.shape = cursor.querySelector(".shape");
    this.cursor.style.opacity = this.visible ? "1" : "0";
    if (this.current) this.applyPoint(this.current);
  }

  async move(x, y) {
    if (document.hidden || !Number.isFinite(x) || !Number.isFinite(y)) return;
    this.mount();
    if (!this.cursor) return;

    this.show();
    this.cancelMotion();
    this.cancelSettle();
    const target = { x, y };
    const start = this.current ?? this.initialPoint();
    const distance = Math.hypot(x - start.x, y - start.y);
    if (distance < 0.5 || this.reducedMotion()) {
      this.current = target;
      this.applyPoint(target);
      return;
    }

    this.applyPoint(start);
    const duration = this.moveDuration(distance);
    const animation = this.cursor.animate(this.frames(start, target, distance, duration), {
      duration,
      easing: "linear",
      fill: "forwards",
    });
    this.motionAnimation = animation;
    try {
      await animation.finished;
    } catch {
      return;
    }
    if (this.motionAnimation !== animation) return;

    this.current = target;
    this.applyPoint(target);
    animation.cancel();
    this.motionAnimation = null;
    this.startSettle();
  }

  show() {
    if (!this.cursor) return;
    const wasVisible = this.visible;
    this.visible = true;
    this.cursor.style.opacity = "1";
    if (wasVisible || this.reducedMotion() || typeof this.shape?.animate !== "function") return;

    this.entranceAnimation?.cancel();
    const animation = this.shape.animate([
      { filter: "blur(5px)", opacity: 0, transform: "scale(.4)" },
      { filter: "blur(0)", opacity: 1, transform: "scale(1)" },
    ], {
      duration: 420,
      easing: "cubic-bezier(.2,.8,.2,1)",
    });
    this.entranceAnimation = animation;
    animation.finished.catch(() => {}).then(() => {
      if (this.entranceAnimation === animation) this.entranceAnimation = null;
    });
  }

  initialPoint() {
    const viewport = this.viewportSize();
    return {
      x: clamp(viewport.width * 0.58, 20, viewport.width - 28),
      y: clamp(viewport.height * 0.55, 20, viewport.height - 32),
    };
  }

  moveDuration(distance) {
    return clamp(distance * 0.9, CURSOR_MIN_MOVE_DURATION_MS, CURSOR_MAX_MOVE_DURATION_MS);
  }

  frames(start, target, distance, duration = this.moveDuration(distance)) {
    const path = this.motionPath(start, target, distance);
    const count = Math.max(16, Math.ceil(duration / 16) + 1);
    const frameSeconds = duration / (count - 1) / 1000;
    let previous = start;
    return Array.from({ length: count }, (_, index) => {
      const offset = index / (count - 1);
      const progress = index === count - 1 ? 1 : springProgress(offset);
      const point = this.pathPoint(path, progress);
      const tangent = this.pathTangent(path, progress);
      const speed = index === 0 ? 0 : Math.hypot(point.x - previous.x, point.y - previous.y) / frameSeconds;
      previous = point;
      return {
        offset,
        transform: this.motionTransform(point, tangent, progress, speed, path.kind),
      };
    });
  }

  motionPath(start, target, distance) {
    const delta = { x: target.x - start.x, y: target.y - start.y };
    const direction = this.unit(delta);
    if (distance <= CURSOR_LONG_MOVE_THRESHOLD) {
      return { kind: "line", start, target, direction };
    }

    const normal = { x: -direction.y, y: direction.x };
    const midpoint = { x: (start.x + target.x) / 2, y: (start.y + target.y) / 2 };
    const bend = Math.min(96, distance * 0.2);
    const positive = { x: midpoint.x + normal.x * bend, y: midpoint.y + normal.y * bend };
    const negative = { x: midpoint.x - normal.x * bend, y: midpoint.y - normal.y * bend };
    const sign = this.edgeClearance(positive) >= this.edgeClearance(negative) ? 1 : -1;
    const offset = { x: normal.x * bend * sign, y: normal.y * bend * sign };
    return {
      kind: "curve",
      start,
      target,
      control1: this.clampControl({
        x: start.x + direction.x * distance * 0.34 + offset.x * 0.65,
        y: start.y + direction.y * distance * 0.34 + offset.y * 0.65,
      }),
      control2: this.clampControl({
        x: target.x - direction.x * distance * 0.2 + offset.x,
        y: target.y - direction.y * distance * 0.2 + offset.y,
      }),
    };
  }

  pathPoint(path, progress) {
    if (path.kind === "line") {
      return {
        x: path.start.x + (path.target.x - path.start.x) * progress,
        y: path.start.y + (path.target.y - path.start.y) * progress,
      };
    }
    const inverse = 1 - progress;
    return {
      x: inverse ** 3 * path.start.x
        + 3 * inverse ** 2 * progress * path.control1.x
        + 3 * inverse * progress ** 2 * path.control2.x
        + progress ** 3 * path.target.x,
      y: inverse ** 3 * path.start.y
        + 3 * inverse ** 2 * progress * path.control1.y
        + 3 * inverse * progress ** 2 * path.control2.y
        + progress ** 3 * path.target.y,
    };
  }

  pathTangent(path, progress) {
    if (path.kind === "line") return path.direction;
    const inverse = 1 - progress;
    return this.unit({
      x: 3 * inverse ** 2 * (path.control1.x - path.start.x)
        + 6 * inverse * progress * (path.control2.x - path.control1.x)
        + 3 * progress ** 2 * (path.target.x - path.control2.x),
      y: 3 * inverse ** 2 * (path.control1.y - path.start.y)
        + 6 * inverse * progress * (path.control2.y - path.control1.y)
        + 3 * progress ** 2 * (path.target.y - path.control2.y),
    });
  }

  motionTransform(point, tangent, progress, speed, kind) {
    const axis = Math.atan2(tangent.y, tangent.x) * 180 / Math.PI;
    const motionEnvelope = smoothstep(0, 0.12, progress) * (1 - smoothstep(0.72, 1, progress));
    let rotation = 0;
    let axisStretch = 1;
    let speedStretch = 1;
    if (kind === "line") {
      const directionBias = clamp(tangent.x * 0.75 - tangent.y * 0.62, -1, 1);
      rotation = directionBias * 70 * Math.sin(progress * Math.PI);
      axisStretch = 1 - 0.15 * Math.sin(progress * Math.PI);
    } else {
      rotation = normalizeAngle(axis + 135) * motionEnvelope;
      const compressed = clamp(1 - speed / 5500, 0.65, 1);
      speedStretch = 1 - (1 - compressed) * Math.sin(progress * Math.PI);
    }
    return [
      `translate3d(${round(point.x - CURSOR_HOTSPOT.x)}px, ${round(point.y - CURSOR_HOTSPOT.y)}px, 0)`,
      `rotate(${round(axis)}deg)`,
      `scale(1, ${round(axisStretch)})`,
      `rotate(${round(-axis)}deg)`,
      `rotate(${round(rotation)}deg)`,
      `scale(${round(speedStretch)}, 1)`,
    ].join(" ");
  }

  startSettle() {
    if (this.reducedMotion() || typeof this.pointer?.animate !== "function") return;
    this.cancelSettle();
    const animation = this.pointer.animate(this.settleFrames(), {
      duration: CURSOR_SETTLE_DURATION_MS,
      easing: "linear",
    });
    this.settleAnimation = animation;
    animation.finished.catch(() => {}).then(() => {
      if (this.settleAnimation === animation) this.settleAnimation = null;
    });
  }

  settleFrames() {
    const count = 48;
    return Array.from({ length: count }, (_, index) => {
      const progress = index / (count - 1);
      const seconds = progress * CURSOR_SETTLE_DURATION_MS / 1000;
      const envelope = Math.sin(progress * Math.PI);
      const sway = Math.sin(seconds / CURSOR_SETTLE_PERIOD_SECONDS * Math.PI * 2);
      return {
        offset: progress,
        transform: `rotate(${round(envelope * sway * CURSOR_SETTLE_ANGLE_DEGREES)}deg)`,
      };
    });
  }

  viewportSize() {
    return {
      height: window.visualViewport?.height ?? window.innerHeight,
      width: window.visualViewport?.width ?? window.innerWidth,
    };
  }

  edgeClearance(point) {
    const viewport = this.viewportSize();
    return Math.min(point.x, point.y, viewport.width - point.x, viewport.height - point.y);
  }

  clampControl(point) {
    const viewport = this.viewportSize();
    return {
      x: clamp(point.x, 20, Math.max(20, viewport.width - 20)),
      y: clamp(point.y, 20, Math.max(20, viewport.height - 20)),
    };
  }

  unit(vector) {
    const length = Math.hypot(vector.x, vector.y);
    if (length < 0.001) return { x: 1, y: 0 };
    return { x: vector.x / length, y: vector.y / length };
  }

  applyPoint(point) {
    if (!this.cursor) return;
    this.cursor.style.transform = [
      `translate3d(${round(point.x - CURSOR_HOTSPOT.x)}px, ${round(point.y - CURSOR_HOTSPOT.y)}px, 0)`,
      "rotate(0deg) scale(1)",
    ].join(" ");
  }

  reducedMotion() {
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  }

  cancelMotion() {
    this.motionAnimation?.cancel();
    this.motionAnimation = null;
  }

  cancelSettle() {
    this.settleAnimation?.cancel();
    this.settleAnimation = null;
  }

  cancelAnimations() {
    this.cancelMotion();
    this.cancelSettle();
    this.entranceAnimation?.cancel();
    this.entranceAnimation = null;
  }

  hide() {
    this.cancelAnimations();
    this.visible = false;
    this.current = null;
    if (this.cursor) this.cursor.style.opacity = "0";
  }
}

const cursorOverlay = new NexusCursorOverlay();
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type === CURSOR_MOVE_MESSAGE) {
    cursorOverlay.move(Number(message.x), Number(message.y)).then(
      () => sendResponse({ ok: true }),
      () => sendResponse({ ok: false }),
    );
    return true;
  }
  if (message?.type === CURSOR_HIDE_MESSAGE) {
    cursorOverlay.hide();
    sendResponse({ ok: true });
  }
  return false;
});
