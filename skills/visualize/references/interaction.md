# Interactive widgets

- Begin with visible, meaningful markup. JavaScript enhances it after streaming; it must not be required to reveal the entire widget.
- Use native `button`, `input`, `select`, and `range` controls with explicit labels. Every visible control must change the visual immediately and support keyboard input.
- Keep one plain state object. Derive displayed values from it, then render through short idempotent functions.
- Resolve elements before binding events. Never call `.addEventListener` directly on `getElementById(...)` or `querySelector(...)`: fail with a clear `Missing widget element: <selector>` error for required elements, and use optional chaining only for genuinely optional controls.
- Bind events once with `addEventListener`. Do not wait for `DOMContentLoaded`; final widget scripts already run after the submitted markup is inserted. Do not mix inline handlers, duplicated listeners, and global mutable callbacks.
- Prefer changing `textContent`, attributes, classes, SVG paths, or chart data over replacing a large subtree with `innerHTML`.
- Animate the visualization, not the surrounding UI. Use 150-400ms transitions and honor `prefers-reduced-motion`.
