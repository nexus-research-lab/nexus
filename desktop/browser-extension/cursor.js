const CURSOR_MOVE_MESSAGE = "NEXUS_CURSOR_MOVE";
const CURSOR_HIDE_DELAY_MS = 1600;
const CURSOR_LONG_MOVE_DISTANCE = 196;

class NexusCursorOverlay {
  constructor() {
    this.host = null;
    this.cursor = null;
    this.current = null;
    this.animation = null;
    this.hideTimer = null;
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
        filter: drop-shadow(0 0 5px rgba(56, 189, 248, .85)) drop-shadow(0 0 13px rgba(139, 92, 246, .5));
        height: 30px;
        left: 0;
        opacity: 0;
        pointer-events: none;
        position: absolute;
        top: 0;
        transform-origin: 3px 3px;
        transition: opacity 160ms ease;
        width: 28px;
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
      <svg viewBox="0 0 28 30" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <linearGradient id="nexus-cursor-stroke" x1="2" y1="2" x2="24" y2="27" gradientUnits="userSpaceOnUse">
            <stop stop-color="#38bdf8"/>
            <stop offset=".55" stop-color="#6366f1"/>
            <stop offset="1" stop-color="#d946ef"/>
          </linearGradient>
        </defs>
        <path d="M3 2.5v21l5.8-5.7 4.7 10 5-2.4-4.8-9.7h8.2L3 2.5Z" fill="white" stroke="url(#nexus-cursor-stroke)" stroke-width="2.2" stroke-linejoin="round"/>
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

    clearTimeout(this.hideTimer);
    this.cursor.style.opacity = "1";
    const target = { x, y };
    const distance = this.current ? Math.hypot(x - this.current.x, y - this.current.y) : 0;
    if (!this.current || distance < 0.5 || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      this.animation?.cancel();
      this.current = target;
      this.applyPoint(target);
      this.scheduleHide();
      return;
    }

    const animation = this.cursor.animate(this.frames(this.current, target, distance), {
      duration: Math.min(460, Math.max(140, distance * 0.75)),
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
    this.scheduleHide();
  }

  frames(start, target, distance) {
    const count = distance > CURSOR_LONG_MOVE_DISTANCE ? 12 : 2;
    const control = {
      x: (start.x + target.x) / 2,
      y: Math.max(12, Math.min(start.y, target.y) - Math.min(90, distance * 0.18)),
    };
    return Array.from({ length: count }, (_, index) => {
      const progress = index / (count - 1);
      const inverse = 1 - progress;
      const x = count === 2
        ? start.x + (target.x - start.x) * progress
        : inverse * inverse * start.x + 2 * inverse * progress * control.x + progress * progress * target.x;
      const y = count === 2
        ? start.y + (target.y - start.y) * progress
        : inverse * inverse * start.y + 2 * inverse * progress * control.y + progress * progress * target.y;
      const scale = 1 + Math.sin(Math.PI * progress) * 0.06;
      return { transform: `translate3d(${x}px, ${y}px, 0) scale(${scale})` };
    });
  }

  applyPoint(point) {
    this.cursor.style.transform = `translate3d(${point.x}px, ${point.y}px, 0)`;
  }

  scheduleHide() {
    clearTimeout(this.hideTimer);
    this.hideTimer = setTimeout(() => {
      this.hide();
    }, CURSOR_HIDE_DELAY_MS);
  }

  hide() {
    clearTimeout(this.hideTimer);
    this.animation?.cancel();
    this.animation = null;
    if (this.cursor) this.cursor.style.opacity = "0";
  }
}

const cursorOverlay = new NexusCursorOverlay();
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.type !== CURSOR_MOVE_MESSAGE) return false;
  cursorOverlay.move(Number(message.x), Number(message.y)).then(
    () => sendResponse({ ok: true }),
    () => sendResponse({ ok: false }),
  );
  return true;
});
