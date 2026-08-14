---
name: visualize
title: Interactive Visualization
description: 在 Nexus 对话中生成交互式图表、流程图、模拟器、可探索讲解、数据看板、视觉对比、SVG/Canvas 艺术和 UI 原型。仅在可视化比正文或表格更清楚时使用；不用于需要持久保存或下载的 HTML、图解文件。
scope: any
tags: [visualization, interactive, chart, diagram, widget]
---

# Nexus Generative UI

Create a custom interactive visual only when it communicates the answer better than ordinary prose or a table.

## Workflow

1. Call `show_widget` with a concise title and one self-contained HTML fragment in `widget_code`.
2. Put explanation and conclusions in the normal response around the widget. Do not repeat the visual as Markdown.
3. The tool result only confirms delivery to the client. Do not claim that rendering succeeded.

## Widget contract

- Return a fragment only. Do not include `html`, `head`, or `body` tags.
- Inline the widget's CSS and JavaScript. SVG, Canvas, DOM, Web Components, and external CDN libraries are supported.
- Network access and external resources are allowed without a domain allowlist. Prefer established HTTPS CDNs.
- The fragment runs in an isolated iframe and cannot access the Nexus page, cookies, storage, or parent DOM.
- Streaming order is short style, visible content, then scripts last. Scripts run only after the complete tool input arrives. Keep native controls and static content useful before initialization.
- Before calling `show_widget`, check every inline script for unmatched quotes, backticks, brackets, and incomplete blocks. Prefer short functions over one monolithic script.
- Separate source data, derived calculations, rendering, and event binding. Avoid constructing large interfaces through nested template literals or long HTML string concatenation; update existing DOM nodes when practical.
- Use the smallest implementation that fully answers the request. Split independent visuals into separate `show_widget` calls with short prose between them, but keep one cohesive widget when its views share state.
- Make the layout responsive at 320px width and avoid fixed viewport dimensions.
- Use accessible labels, keyboard-operable controls, visible focus, and reduced-motion fallbacks.
- Keep interaction local to the widget. Do not assume a host API or `postMessage` bridge.

## Nexus visual language

- Make the visual feel native to the surrounding answer: flat, quiet, compact, and content-first.
- Keep the outermost background transparent. Use neutral surfaces, one-pixel borders, and 8px or 12px radii; avoid gradients, glass, glow, heavy shadows, and decorative hero layouts.
- Use only 400 and 500 font weights. Body text is 14px to 16px, labels are at least 12px, and controls are compact rather than oversized.
- Use the accent color only for selection, focus, or the primary data series. Prefer one neutral ramp and at most two categorical color ramps.
- Do not recreate Nexus chrome or add a second page title inside the widget. `widget_code` contains the visual and its local controls, not duplicate prose.
- Let content determine height. Do not create nested scrolling, fixed-position overlays, or viewport-sized shells.

## Theme variables

- `--nexus-background`
- `--nexus-surface`
- `--nexus-surface-hover`
- `--nexus-text`
- `--nexus-muted`
- `--nexus-border`
- `--nexus-accent`
- `--nexus-accent-contrast`
- `--nexus-chart-1` through `--nexus-chart-5`
- `--nexus-font-sans`
- `--nexus-font-mono`
- `--nexus-radius-md`
- `--nexus-radius-lg`

Use these variables for CSS and SVG. Canvas APIs cannot resolve CSS `var(...)` strings; read their computed values first.

## Interactive widgets

- Begin with visible, meaningful markup. JavaScript enhances it after streaming; it must not be required to reveal the entire widget.
- Use native `button`, `input`, `select`, and `range` controls with explicit labels. Every visible control must change the visual immediately and support keyboard input.
- Keep one plain state object. Derive displayed values from it, then render through short idempotent functions.
- Bind events once with `addEventListener`. Do not mix inline handlers, duplicated listeners, and global mutable callbacks.
- Prefer changing `textContent`, attributes, classes, SVG paths, or chart data over replacing a large subtree with `innerHTML`.
- Animate the visualization, not the surrounding UI. Use 150-400ms transitions and honor `prefers-reduced-motion`.

## Charts

- Use SVG or native DOM for small charts. Use Chart.js only when axes, tooltips, or multiple dynamic series justify it.
- Wrap each canvas in a `position:relative` container with an explicit height. Do not set CSS height on canvas. Use `responsive:true` and `maintainAspectRatio:false`.
- Canvas cannot resolve CSS variables. Read `--nexus-chart-1` through `--nexus-chart-5` with `getComputedStyle(document.documentElement).getPropertyValue(name).trim()`.
- Assigning `canvas.width` or `canvas.height` clears the bitmap and resets the context. Set the backing size only during initialization or a real resize, never inside draw or coordinate helpers.
- Give every canvas a unique id. Keep the chart instance and guard initialization so CDN `onload` plus an immediate fallback cannot create it twice.
- Load established UMD builds over HTTPS. Put the library script before the initializer, use `onload` to call a named init function, and also call it when the global already exists.
- Controls must update chart data and call `chart.update()`. Disable library legends when a compact HTML legend communicates values more clearly.
- Round displayed values consistently, label axes and units, and pad plot ranges so points and labels are not clipped.

## Diagrams

- Prefer one responsive SVG with `width="100%"` and a complete `viewBox`. Put `defs` and arrow markers before visible nodes so streaming connectors are valid.
- Choose the structure that matches the idea: flow for sequence, hierarchy for ownership, cycle for feedback, matrix for two dimensions, timeline for change, or side-by-side for comparison.
- Keep node titles to five words when possible and at most four full-size nodes per row. Put detail in surrounding prose or an interactive inspector.
- Calculate the `viewBox` from the lowest element plus padding. Keep labels inside bounds and account for `text-anchor` direction.
- Connect edges from node boundaries, use a shared marker, and verify no edge crosses unrelated nodes or text.
- Use neutral structure plus no more than two categorical chart colors. Encode status with text or shape as well as color.
- For interactive diagrams, mutate classes and SVG attributes on existing elements instead of rebuilding the SVG.

Use `diagram-design` instead when the user wants a persistent, downloadable single-file HTML diagram rather than an inline conversational widget.

## UI mockups

- Reproduce the requested product surface, not an entire decorative landing page. Omit browser chrome, fake sidebars, duplicate titles, and ornamental hero areas unless they are the subject.
- Use a clear reading order: compact controls, primary content, then secondary detail. Prefer dividers and whitespace over nested cards.
- Use CSS Grid for comparable metrics and Flexbox for compact controls. Keep labels and values aligned to common axes.
- Use one-pixel `--nexus-border` boundaries, `--nexus-surface` for restrained grouping, and 8px or 12px radii. Avoid gradients, glass, glow, and heavy shadows.
- Empty, loading, selected, warning, and error states must remain distinguishable in both light and dark themes.
- On narrow widths, reflow columns and allow tables or timelines to scroll only when their data cannot remain legible otherwise.

## SVG and Canvas art

- Prefer SVG for illustrations and finite diagrams; use Canvas for continuous animation, dense particles, or pixel-level drawing.
- For Canvas, initialize the backing store once per actual size change, scale for `devicePixelRatio` once, and keep coordinate conversion separate from drawing.
- Resolve Nexus color variables to concrete values before assigning `fillStyle`, `strokeStyle`, shadows, or gradients.
- Run animation through `requestAnimationFrame`, cap particle or object counts, and pause or simplify when `prefers-reduced-motion` is enabled.
- Keep controls and captions as accessible HTML outside Canvas. Do not make essential meaning depend on pixels alone.
- Keep the outer surface transparent and let the artwork, not decorative containers, carry the composition.
