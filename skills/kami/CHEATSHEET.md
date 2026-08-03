# kami · Cheatsheet

One-page quick reference. Scan before filling a template or tweaking a detail. Full spec in `references/design.md`.

## Ten invariants

1. Page background `#f5f4ed` (parchment), never pure white
2. Single accent: ink-blue `#1B365D`
3. All grays **warm-toned** (yellow-brown undertone), no cool blue-gray
4. One serif font per page (headings + body). `--sans` is a CSS alias for the same family; introduce a real sans only for genuinely UI-style chrome
5. Serif weight locked at 500, no bold
6. Line-height: headlines 1.1-1.3 / dense 1.4-1.45 / reading 1.5-1.55
7. Letter-spacing: Chinese body with TsangerJinKai 0.1-0.2pt (dense layouts may push to 0.3pt); English body 0; small labels and all-caps overlines get +0.2-1pt
8. Tag backgrounds solid hex, no rgba (WeasyPrint double-rectangle bug)
9. Depth via ring / whisper shadow, no hard drop shadows
10. No italic in templates or demos

## Sources and Materials

Full pass in SKILL.md Step 2.1. The one contract worth repeating: a number you cannot verify ships as a magnitude or a marked gap, never as fake precision.

## Color


| Role         | Hex           | Use                                                 |
| ------------ | ------------- | --------------------------------------------------- |
| Parchment    | `#f5f4ed`     | Page background                                     |
| Ivory        | `#faf9f5`     | Card / lifted container                             |
| Warm Sand    | `#e8e6dc`     | Button / interactive surface                        |
| Dark Surface | `#30302e`     | Dark container                                      |
| Deep Dark    | `#141413`     | Dark page background                                |
| **Brand**    | **`#1B365D`** | **Accent · CTA · title left bar (≤ 5% of surface)** |
| Ink Light    | `#2D5A8A`     | Links on dark surfaces                              |
| Near Black   | `#141413`     | Primary text                                        |
| Dark Warm    | `#3d3d3a`     | Secondary text · table headers · links              |
| Olive        | `#504e49`     | Subtext · descriptions                              |
| Stone        | `#6b6a64`     | Tertiary · metadata                                 |
| Border       | `#e8e6dc`     | Primary border · section divider                    |
| Border Soft  | `#e5e3d8`     | Secondary border · row separator                    |


**rgba -> solid** (parchment base + ink-blue):


| Alpha    | Solid                       |
| -------- | --------------------------- |
| 0.08     | `#EEF2F7`                   |
| 0.14     | `#E4ECF5`                   |
| **0.18** | **`#E4ECF5`** ← default tag |
| 0.22     | `#D0DCE9`                   |
| 0.30     | `#D6E1EE`                   |


## Type (print pt)


| Role       | Size | Weight | Line-height |
| ---------- | ---- | ------ | ----------- |
| Display    | 36   | 500    | 1.10        |
| H1         | 22   | 500    | 1.20        |
| H2         | 16   | 500    | 1.25        |
| H3         | 13   | 500    | 1.30        |
| Body Lead  | 11   | 400    | 1.55        |
| Body       | 10   | 400    | 1.55        |
| Body Dense | 9.2  | 400    | 1.42        |
| Caption    | 9    | 400    | 1.45        |
| Label      | 9    | 600    | 1.35        |
| Tiny       | 9    | 400    | 1.40        |


Screen (px) ≈ pt × 1.33.
Minimum floor: web text >= 12px, PDF text >= 9pt.

## Font stacks

Each language uses a single serif for the entire page. `--sans` always equals `var(--serif)`.

English:

```css
--serif: Charter, Georgia, Palatino,
         "Times New Roman", serif;
--sans:  var(--serif);
--mono:  "JetBrains Mono", "SF Mono", "Fira Code",
         Consolas, Monaco, monospace;
```

Chinese:

```css
--serif: "TsangerJinKai02", "Source Han Serif SC",
         "Noto Serif CJK SC", "Songti SC", "STSong",
         Georgia, serif;
--sans:  var(--serif);
--mono:  "JetBrains Mono", "SF Mono", Consolas,
         "TsangerJinKai02", "Source Han Serif SC",
         monospace;
```

Japanese:

```css
--serif: "YuMincho", "Yu Mincho", "Hiragino Mincho ProN",
         "Noto Serif CJK JP", "Source Han Serif JP",
         "TsangerJinKai02", Georgia, serif;
--sans:  var(--serif);
```

Any font-family that may render Chinese or Japanese must include a CJK fallback, including `@page` footer text, `pre`, `code`, and SVG labels. A pure mono stack can render missing glyph boxes in WeasyPrint.

## Spacing (4pt base)


| Tier | Value    | Use                    |
| ---- | -------- | ---------------------- |
| xs   | 2-3pt    | Inline                 |
| sm   | 4-5pt    | Tag padding            |
| md   | 8-10pt   | Component interior     |
| lg   | 16-20pt  | Between components     |
| xl   | 24-32pt  | Section-title margin   |
| 2xl  | 40-60pt  | Between major sections |
| 3xl  | 80-120pt | Between chapters       |


**Page margins (A4)**


| Document      | T · R · B · L        |
| ------------- | -------------------- |
| Resume        | 11 · 13 · 11 · 13 mm |
| One-Pager     | 15 · 18 · 15 · 18 mm |
| Long Doc      | 20 · 22 · 22 · 22 mm |
| Letter        | 25 mm all sides      |
| Portfolio     | 12 · 15 · 12 · 15 mm |
| Equity Report | 16 · 18 · 18 · 18 mm |
| Changelog     | 20 · 22 · 22 · 22 mm |
| Landing Page  | N/A (screen-first, max-width: 1120px, padding: 88px 64px) |


## Radius scale

`4pt -> 6pt -> 8pt (default) -> 12pt -> 16pt -> 24pt -> 32pt (hero)`

## Common CSS snippets

### Card

```css
.card {
  background: var(--ivory);       /* the fill IS the lift; no closed border */
  border-radius: 4pt;
  padding: 16pt 20pt;
}
```

A sub-1pt closed border plus a radius renders as a double ring (production.md
pitfall #2) and fails `scripts/build.py --check`. To give a card more weight,
mark one edge with `border-left: 1.4pt solid var(--brand)`.

### Tag (solid fill, never rgba)

```css
.tag {
  background: var(--tag-bg);
  color: var(--brand);
  font-size: 9pt; font-weight: 500;
  padding: 1pt 5pt;
  border-radius: 3pt;
  letter-spacing: 0.3pt;
}
```

### Section title

```css
h2 {
  font-family: var(--serif);
  font-size: 16pt; font-weight: 500;
  color: var(--near-black);
  margin-bottom: 6pt;
}
```

Type carries the hierarchy; a section head needs no rule, bar, or underline.
`changelog*.html` is the one template that adds `border-left: 2.5pt solid
var(--brand)` to `h2`, because release notes are scanned for group boundaries
rather than read straight through. Do not carry that bar into other documents:
repeated down a page it turns every heading into a container and is the single
most common way a kami document stops looking like one. `resume*.html` uses a
quiet bottom rule, and keeps project rows borderless so section titles never
create double rules or lonely page-top lines.

### Table (kami-table)

Base class works on bare `<table>` or `.kami-table`. Add variant classes for density/alignment:

```css
/* Base */
table, .kami-table {
  width: 100%; border-collapse: collapse;
  font-size: 9.5pt; margin: 12pt 0; break-inside: avoid;
}
table th { text-align: left; font-weight: 500; color: var(--dark-warm);
  padding: 6pt 8pt; border-bottom: 1pt solid var(--border); }
table td { padding: 5pt 8pt; border-bottom: 0.3pt solid var(--border-soft);
  vertical-align: top; }
```


| Variant   | Class              | Effect                                               |
| --------- | ------------------ | ---------------------------------------------------- |
| Compact   | `.compact`         | 8pt font, tight padding (data-dense tables)          |
| Financial | `.financial`       | Right-align all columns except first, `tabular-nums` |
| Striped   | `.striped`         | Alternating `var(--ivory)` row background            |
| Total row | `.total` on `<tr>` | Bold, brand top border, no bottom border             |


Combine freely: `<table class="kami-table financial striped">`.

### Metric (data card)

```css
.metric { display: flex; align-items: baseline; gap: 6pt; }
.metric-value {
  font-family: var(--serif); font-size: 16pt; font-weight: 500;
  color: var(--brand);
  font-variant-numeric: tabular-nums;
}
.metric-label { font-size: 9pt; color: var(--olive); }
```

### Quote

```css
.quote {
  border-left: 2pt solid var(--brand);
  padding: 4pt 0 4pt 14pt;
  color: var(--olive);
  line-height: 1.55;
}
```

## Diagram components

Eighteen built-in diagram types (incl. Mermaid-sourced sequence / class / ER; see `references/mermaid.md`). Extract the `<svg>` block and embed in a `<figure>` in long-doc / portfolio:


| Type          | File                                 | Use                                             |
| ------------- | ------------------------------------ | ----------------------------------------------- |
| Architecture  | `assets/diagrams/architecture.html`  | System components and connections               |
| Architecture Board | `assets/diagrams/architecture-board.html` | Report-scale five-layer system board (standalone page) |
| Flowchart     | `assets/diagrams/flowchart.html`     | Decision branches and flows                     |
| Quadrant      | `assets/diagrams/quadrant.html`      | 2×2 positioning                                 |
| Bar Chart     | `assets/diagrams/bar-chart.html`     | Category comparison (up to 8 groups × 3 series) |
| Line Chart    | `assets/diagrams/line-chart.html`    | Trends over time (up to 12 points × 3 lines)    |
| Donut Chart   | `assets/diagrams/donut-chart.html`   | Proportional breakdown (up to 6 segments)       |
| State Machine | `assets/diagrams/state-machine.html` | Finite states + directed transitions            |
| Timeline      | `assets/diagrams/timeline.html`      | Time axis + milestone events                    |
| Swimlane      | `assets/diagrams/swimlane.html`      | Cross-responsibility process flow               |
| Tree          | `assets/diagrams/tree.html`          | Hierarchical relationships                      |
| Layer Stack   | `assets/diagrams/layer-stack.html`   | Vertically stacked system layers                |
| Venn          | `assets/diagrams/venn.html`          | Set intersections and overlaps                  |
| Candlestick   | `assets/diagrams/candlestick.html`   | OHLC price history (up to 30 days)              |
| Waterfall     | `assets/diagrams/waterfall.html`     | Revenue bridge / decomposition                  |


Usage: extract the `<svg>` block from the HTML file and paste into the template's `<figure>` container.

**Repo-maintained diagram** (README / docs-site figure living in the user's repository): keep the trio consistent, `index.html` source + same-name PNG re-exported after every change + `prompt.md` (must preserve / suggested additions / visual direction / sister boundaries). Evidence pass before drawing; maturity encoding for shipped / in-build / future. See `references/diagrams.md` «Maintained diagram assets».

**Data chart colors**: primary series `#1B365D` · secondary `#504e49` → `#6b6a64` → `#b8b7b0` → `#d4d3cd` → `#EEF2F7`.

**Editing data**: only modify elements between `<!-- DATA START -->` / `<!-- DATA END -->`, leave CSS untouched. All coordinates must be divisible by 4.

## Dark section

Alternate light/dark rhythm: add `.sd-alt` to any section container.

- Background switches to `--deep-dark` (`#141413`)
- Body text switches to `--warm-silver` (`#b0aea5`)
- Headings switch to `--ivory`
- Appropriate for: section-level light/dark alternation in long-doc / portfolio
- Restriction: showcase pages only, never in print templates

## Verification checks

`python3 scripts/build.py --verify [target]` covers render, page count, font embedding, and PPTX generation for source templates and slides.

Source templates intentionally keep `{{...}}` fields. Run `python3 scripts/build.py --check-placeholders path/to/filled.html` on completed documents. Run `python3 scripts/build.py --check-density` to warn on pages with >25% trailing whitespace (skips cover).

For new documents built from raw material, validate the content IR before layout and re-check coverage after filling: `python3 scripts/build.py --check-content content.json [filled.html]` (schemas in `references/schemas/`). Before shipping a filled PDF, run `python3 scripts/build.py --check-visual path/to/filled.pdf` and view every exported page image against the printed checklist.

Marp variant deck (opt-in): `assets/templates/marp/`. Render with local `marp-cli`. See design.md §8 + production.md Part 2.5.

## Content quality (one rule per type)

Full quality bars in `references/writing.md`. The single most important rule for each document type:


| Document      | Core quality rule                                                                    |
| ------------- | ------------------------------------------------------------------------------------ |
| Resume        | Every bullet: Action + Scope + Measurable Result + Business Outcome                  |
| Portfolio     | Open with the problem and stakes, not the project name                               |
| Slides        | Slide titles are full sentences (assertions), not topic labels                       |
| Equity Report | Lead with variant perception: what you see that the market doesn't                   |
| Long Document | Each chapter claim paragraph must survive the "so what?" test                        |
| One-Pager     | Metrics are the headline; if the 4 cards don't tell the story, the metrics are wrong |
| Letter        | First paragraph states purpose in one sentence                                       |
| Changelog     | One sentence per change, verb-led, user-facing language                              |


## Per-page font size strategy (Resume two-page)

Page 1 carries the projects section, which is the densest content. Page 2 carries open source, convictions, impact, skills, and education, which has more breathing room.

| Location | Class | Default | Dense (5 projects) |
|---|---|---|---|
| Project body | `.proj-text` | 9pt / lh 1.40 | 9pt / lh 1.38 |
| Timeline body | `.tl-body` | 9pt / lh 1.40 | 8.5pt (CN) |
| Summary | `.summary` | 9.2pt | 9pt via body |
| Section titles | `.section-title` | margin-top 5mm | 3.5mm |
| OS intro | `.os-intro` | 9.2pt | unchanged |
| Conviction body | `.conv-body` | 9pt | unchanged |
| Skills body | `.skill-body` | 9pt | unchanged |

**Reference config (5 projects + full page 2)**:

```html
<body class="resume--dense">
  <!-- page 1: 5 projects, timeline, summary -->
  <!-- page 2: OS grid (6), convictions, impact, skills, edu -->
</body>
```

Filled resume PDFs should be exactly 2 pages with both pages visually used. Check the rendered result:

```bash
python3 scripts/build.py --check-resume-balance path/to/resume.pdf
```

Page 2 font sizes stay at template defaults. The density variant only tightens page 1 elements. If page 2 has unusually long content, reduce `.os-intro`, `.conv-body`, or `.skill-body` individually, never globally.

Resume visual rule: header and section titles carry the only structural rules. Top metrics stack value over label so labels stay single-line; project rows separate by padding, not borders.

## Quick decisions


| Need                | Use                                                            |
| ------------------- | -------------------------------------------------------------- |
| Headline            | serif 500, line-height 1.10-1.30                               |
| Reading body        | serif 400, 9.5-10pt, 1.55 (CN pins `--sans: var(--serif)`)     |
| Emphasize a number  | `color: var(--brand)`, no bold                                 |
| Raise a passage     | `.callout`: ivory fill + 2pt brand left rule + 3pt radius        |
| Quote               | same 2pt left rule + olive, no fill (fill is what makes a callout) |
| Code                | `long-doc` `pre` / `code`: ivory fill, 4pt / 2pt radius, no border |
| Key figures         | `one-pager` `.metric`: baseline row, transparent, not a card    |
| Buttons             | `landing-page` `.btn-primary` / `.btn-ghost` (screen only)      |
| Section start       | `long-doc` `h2`: serif, no bar (`changelog` `h2` is the exception) |
| Cover               | Display heading + right-aligned author/date + heavy whitespace |
| Figure SVG          | `width: 100%; height: auto; max-height: <safe>`. Never `max-height` alone (starves width on wide viewBoxes; production.md #17). |
| Metric labels (4-col) | Soft cap 14-18 chars at 9pt Charter; trim context, don't wrap (production.md #18). |
| Multi-column body   | Hold lengths within ±10 chars across parallel columns (production.md #19). |
| Image references    | Always inside `assets/demos/images/` or `assets/illustrations/`; never `../../sibling-project/...` (production.md #20). |
| Metric row layout   | Vertical stack (`flex-direction: column`); horizontal baseline-align breaks when any label wraps (production.md #21). |
| Slide bullets       | Numerals `1. 2. 3.` or `•`; en-dash `–` reads informal at slide scale (production.md #22). Print docs keep en-dash. |


Not on the table -> first principles: **serif carries authority, sans carries utility, warm gray carries rhythm, ink-blue carries focus**.
