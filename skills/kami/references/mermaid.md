# Mermaid Diagrams

Kami turns Mermaid text into editorial, Kami-styled diagrams using
[beautiful-mermaid](https://github.com/lukilabs/beautiful-mermaid) (MIT). Instead
of hand-tuning SVG coordinates, describe a diagram in Mermaid and let the
pipeline produce a parchment / ink-blue diagram that drops into any document.

## Two paths

| Path | Surface | Renderer | Diagram types |
|------|---------|----------|---------------|
| **PDF** | one-pager / long-doc / equity-report etc. | WeasyPrint (no JS) | flowchart, state, sequence, class, ER |
| **Browser** | any page running beautiful-mermaid (e.g. <https://agents.craft.do/mermaid>) | beautiful-mermaid in the browser | all of the above + xychart |

The split exists because WeasyPrint does not execute JavaScript and applies inline
SVG only partially. The graph types carry their colors on inline presentation
attributes, which survive normalization to static hex. `xychart-beta` drives its
colors through `<style>` class selectors instead, which WeasyPrint does not apply
to inline SVG, so charts stay on the browser path. Kami already ships hand-drawn
`bar-chart` / `line-chart` / `donut-chart` / `candlestick` / `waterfall` diagrams
for PDF, so this is no loss of capability.

## The Kami theme

`references/mermaid-theme.json` is the single source of truth mapping
beautiful-mermaid's seven color roles onto the Kami palette (`references/tokens.json`):

| role | token | hex |
|------|-------|-----|
| `bg` | `--parchment` | `#f5f4ed` |
| `fg` | `--near-black` | `#141413` |
| `line` | `--olive` | `#504e49` |
| `accent` | `--brand` | `#1B365D` |
| `muted` | `--stone` | `#6b6a64` |
| `surface` | `--ivory` | `#faf9f5` |
| `border` | `--border` | `#e8e6dc` |

Font resolves to the Kami serif stack (`Charter ... TsangerJinKai02 ...`), so CJK
labels render and embed correctly. Keep these hex values in sync with `tokens.json`.

## Most cases: edit a ready template

Kami ships `sequence`, `class`, and `er` as static Kami-styled diagrams (joining the
15 hand-drawn ones). The common case needs **no tooling at all**: copy the nearest
`assets/diagrams/*.html`, edit the text labels, embed the `<svg>` into your document.
beautiful-mermaid is the *source* of these diagrams; you do not need it to use them.

## New diagram from Mermaid text (no Node)

Kami's build stays pure Python; beautiful-mermaid (a Node package) is never bundled.
To make a new diagram from Mermaid text:

```bash
# 1. Render the Mermaid to SVG in any browser running beautiful-mermaid
#    (e.g. https://agents.craft.do/mermaid). Theme choice does not matter --
#    the normalizer re-themes to the Kami palette. Save the SVG.
# 2. Re-theme + make it WeasyPrint-safe (pure Python, no Node, no network):
python3 scripts/mermaid_normalize.py raw.svg -o clean.svg
# 3. Paste <svg>...</svg> into a diagram shell (copy assets/diagrams/sequence.html),
#    register it in DIAGRAM_TARGETS (scripts/build.py), then verify:
python3 scripts/build.py diagram-<name>     # renders to assets/examples/*.pdf
```

The `.mmd` files in `assets/diagrams/src/` record the Mermaid source of the shipped
diagrams. Do not hand-edit the SVG coordinates in the committed HTML.

## What the normalizer does

`scripts/mermaid_normalize.py` turns any beautiful-mermaid SVG into a Kami diagram:

- **Re-themes** it to the Kami palette by overriding the seven root color roles from
  `references/mermaid-theme.json`, so the source theme is irrelevant.
- **Resolves** every `var()` and `color-mix(in srgb, ...)` to a static hex (WeasyPrint
  does not compute `color-mix()` or reliably cascade SVG `<style>` custom props).
- **Fixes fonts**: strips beautiful-mermaid's Google-Fonts `@import` and rewrites the
  (mis-quoted) `font-family` to the Kami serif stack.

The `--check` lint flags any `color-mix(`, `<foreignObject>`, or web-font import that
reaches a PDF-bound template, so an un-normalized SVG cannot slip in.

## Compatibility notes (verified)

beautiful-mermaid v1.1.3 graph-type output uses native `<text>` (no
`<foreignObject>`), no `filter`, and no `<pattern>`. After normalization the SVG is
fully static hex on inline presentation attributes, which WeasyPrint renders
correctly, including CJK labels with embedded fonts.

## When to use a diagram

A diagram earns its space only when the **structure** is the point. If a table or a
sentence conveys the same relationship, use that instead. Keep diagrams to one
focal idea; the accent (ink-blue) marks the focal element, everything else stays in
warm neutrals. See `references/design.md` and `references/anti-patterns.md`.
