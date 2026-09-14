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

1. Choose the visual form and read only the relevant references below before composing the fragment.
2. Call `show_widget` with a concise title and one self-contained HTML fragment in `widget_code`.
3. Put explanation and conclusions in the normal response around the widget. Do not repeat the visual as Markdown.
4. The tool result only confirms delivery to the client. Do not claim that rendering succeeded.

## Read on demand

Resolve these paths relative to this skill directory. Do not load the whole reference directory. Combine references only when the widget needs both concerns, such as an interactive diagram.

| When building | Read |
| --- | --- |
| Controls, selections, or state-driven views | [Interaction](references/interaction.md) |
| Quantitative charts, axes, or Chart.js | [Charts](references/charts.md) |
| Flows, hierarchies, networks, or explanatory diagrams | [Diagrams](references/diagrams.md) |
| Product UI prototypes, dashboards, or comparison layouts | [UI mockups](references/ui-mockups.md) |
| SVG illustrations, Canvas drawing, or continuous animation | [Art](references/art.md) |

## Widget contract

- Return a fragment only. Do not include `html`, `head`, or `body` tags.
- Inline the widget's CSS and JavaScript. SVG, Canvas, DOM, Web Components, and external CDN libraries are supported.
- Network access and external resources are allowed without a domain allowlist. Prefer established HTTPS CDNs.
- The fragment runs in an isolated iframe and cannot access the Nexus page, cookies, storage, or parent DOM.
- Streaming order is short style, visible content, then scripts last. Scripts run only after the complete tool input arrives. Keep native controls and static content useful before initialization.
- Before calling `show_widget`, check every inline script for unmatched quotes, backticks, brackets, and incomplete blocks. Prefer short functions over one monolithic script.
- Before calling `show_widget`, verify that every id, class, or data attribute referenced by JavaScript exists in the submitted markup and is spelled identically.
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
