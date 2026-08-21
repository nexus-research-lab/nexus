const CURSOR_MOVE_MESSAGE = "NEXUS_CURSOR_MOVE";
const CURSOR_HIDE_MESSAGE = "NEXUS_CURSOR_HIDE";
const CURSOR_MIN_MOVE_DURATION_MS = 240;
const CURSOR_MAX_MOVE_DURATION_MS = 680;

class NexusCursorOverlay {
  constructor() {
    this.host = null;
    this.cursor = null;
    this.current = null;
    this.animation = null;
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
    this.animation?.cancel();

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
        filter: drop-shadow(0 2px 2px rgba(15, 23, 42, .5)) drop-shadow(0 0 14px rgba(15, 23, 42, .32));
        height: 29px;
        left: 0;
        opacity: 0;
        pointer-events: none;
        position: absolute;
        top: 0;
        transition: opacity 160ms ease;
        width: 24px;
        will-change: transform, opacity;
      }
      .cursor svg { display: block; height: 100%; width: 100%; }
      .pointer {
        animation: nexus-cursor-idle 1.8s ease-in-out infinite;
        height: 100%;
        transform-origin: 4px 4px;
        width: 100%;
      }
      .cursor.moving .pointer { animation: none; }
      @keyframes nexus-cursor-idle {
        0%, 100% { transform: rotate(-5deg); }
        50% { transform: rotate(5deg); }
      }
      @media print { .cursor { display: none; } }
      @media (prefers-reduced-motion: reduce) {
        .cursor { transition: none; }
        .pointer { animation: none; }
      }
    `;
    const cursor = document.createElement("div");
    cursor.className = "cursor";
    cursor.setAttribute("aria-hidden", "true");
    cursor.innerHTML = `
      <div class="pointer">
        <svg viewBox="0 0 24 29" xmlns="http://www.w3.org/2000/svg">
          <path d="M3.1 2.3 21 15.2c.8.6.4 1.8-.6 1.9l-7.1 1.3-4.4 6.5c-.6.9-1.9.6-2.1-.4L1.4 4.1c-.3-1.1.8-2.4 1.7-1.8Z" fill="#64748b" fill-opacity=".78" stroke="#fff" stroke-width="2.1" stroke-linejoin="round"/>
        </svg>
      </div>
    `;
    shadow.append(style, cursor);
    document.documentElement.appendChild(host);

    this.host = host;
    this.cursor = cursor;
    if (this.current) this.applyPoint(this.current);
  }

  async move(x, y) {
    if (document.hidden || !Number.isFinite(x) || !Number.isFinite(y)) return;
    this.mount();
    if (!this.cursor) return;

    this.cursor.style.opacity = "1";
    const target = { x, y };
    const start = this.current ?? this.initialPoint();
    const distance = Math.hypot(x - start.x, y - start.y);
    if (distance < 0.5 || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      this.animation?.cancel();
      this.cursor.classList.remove("moving");
      this.current = target;
      this.applyPoint(target);
      return;
    }

    this.applyPoint(start);
    this.cursor.classList.add("moving");
    const animation = this.cursor.animate(this.frames(start, target, distance), {
      duration: Math.min(CURSOR_MAX_MOVE_DURATION_MS, Math.max(CURSOR_MIN_MOVE_DURATION_MS, distance * 0.9)),
      easing: "cubic-bezier(.22,.8,.28,1)",
      fill: "forwards",
    });
    this.animation?.cancel();
    this.animation = animation;
    try {
      await animation.finished;
    } catch {
      return;
    }
    if (this.animation !== animation) return;

    this.current = target;
    this.applyPoint(target);
    animation.cancel();
    this.animation = null;
    this.cursor.classList.remove("moving");
  }

  initialPoint() {
    return {
      x: Math.max(20, Math.min(window.innerWidth - 28, window.innerWidth * 0.5)),
      y: Math.max(20, Math.min(window.innerHeight - 32, window.innerHeight * 0.55)),
    };
  }

  frames(start, target, distance) {
    const count = Math.max(8, Math.min(20, Math.ceil(distance / 36)));
    const control = {
      x: (start.x + target.x) / 2,
      y: Math.max(12, Math.min(start.y, target.y) - Math.min(90, distance * 0.18)),
    };
    return Array.from({ length: count }, (_, index) => {
      const progress = index / (count - 1);
      const inverse = 1 - progress;
      const x = inverse * inverse * start.x + 2 * inverse * progress * control.x + progress * progress * target.x;
      const y = inverse * inverse * start.y + 2 * inverse * progress * control.y + progress * progress * target.y;
      return { transform: `translate3d(${x}px, ${y}px, 0)` };
    });
  }

  applyPoint(point) {
    this.cursor.style.transform = `translate3d(${point.x}px, ${point.y}px, 0)`;
  }

  hide() {
    this.animation?.cancel();
    this.animation = null;
    this.cursor?.classList?.remove("moving");
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
