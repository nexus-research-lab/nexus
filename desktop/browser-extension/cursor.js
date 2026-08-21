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
        filter: drop-shadow(0 2px 2px rgba(15, 23, 42, .36)) drop-shadow(0 5px 10px rgba(15, 23, 42, .16));
        height: 29px;
        left: 0;
        opacity: 0;
        pointer-events: none;
        position: absolute;
        top: 0;
        transform-origin: 3px 3px;
        transition: opacity 160ms ease;
        width: 24px;
        will-change: transform, opacity;
      }
      .cursor svg { display: block; height: 100%; width: 100%; }
      @media print { .cursor { display: none; } }
      @media (prefers-reduced-motion: reduce) { .cursor { transition: none; } }
    `;
    const cursor = document.createElement("div");
    cursor.className = "cursor";
    cursor.setAttribute("aria-hidden", "true");
    cursor.innerHTML = `
      <svg viewBox="0 0 24 29" xmlns="http://www.w3.org/2000/svg">
        <path d="M2.5 1.8v21.3l5.7-5.4 4.5 9.5 4.8-2.3-4.5-9.3h8.3L2.5 1.8Z" fill="#fff" stroke="#202124" stroke-width="1.7" stroke-linejoin="round"/>
      </svg>
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
      this.current = target;
      this.applyPoint(target);
      return;
    }

    this.applyPoint(start);
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
