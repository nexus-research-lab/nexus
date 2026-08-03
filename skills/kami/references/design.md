# Design System

## Principles

kami's aesthetic compresses into one sentence: **warm parchment canvas, ink-blue accent, serif carries hierarchy, avoid cool grays and hard shadows**.

This is not a UI framework. It is a constraint system for print, designed to keep pages stable, clear, and readable.

**The ten invariants** (each has a real cost, think before overriding):

1. Page background parchment `#f5f4ed`, never pure white
2. Single accent: ink-blue `#1B365D`, no second chromatic color
3. All grays warm-toned (yellow-brown undertone), no cool blue-grays
4. English: serif for everything (headlines and body). Chinese: serif headlines, sans body. Sans only for UI elements (labels, eyebrows, meta) in both
5. Serif weight locked at 500, no bold
6. Line-heights: tight headlines 1.1-1.3, dense body 1.4-1.45, reading body 1.5-1.55
7. Letter-spacing: Chinese body 0.3pt for comfortable reading; English body 0; tracking only for short labels and overlines
8. Tag backgrounds must be solid hex, never rgba (WeasyPrint renders a double rectangle)
9. Depth via ring shadow or whisper shadow, never hard drop shadows
10. **No italic in print templates**. No `font-style: italic` in any PDF template or demo. Exception: landing page (screen-only) uses italic for poetic lines (gallery captions, feature subtitles, footer ethos)

This system is a fusion of Anthropic's visual language and real Chinese / English resume iteration. Details below.

---

## 1. Color

**Single accent, warm neutrals only, zero cool tones** - this is the core.

### Brand

```css
--brand:       #1B365D;   /* Ink Blue - the only chromatic color. CTAs, accents, section-title left bar. */
--brand-light: #2D5A8A;   /* Ink Light - brighter variant, for links on dark surfaces. */
```

**Rule**: ink-blue covers ≤ **5% of document surface area**. More than that is ornament, not restraint.

### Surface

```css
--parchment:    #f5f4ed;   /* Page background - warm cream, the emotional foundation */
--ivory:        #faf9f5;   /* Card / lifted container - brighter than parchment */
--warm-sand:    #e8e6dc;   /* Button default / interactive surface */
--dark-surface: #30302e;   /* Dark-theme container - warm charcoal */
--deep-dark:    #141413;   /* Dark-theme page background - not pure black, slight olive undertone */
```

**Never**: `#ffffff` pure white as page background. `#f8f9fa` / `#f3f4f6` or any cool-gray surface.

### Text

```css
--near-black:  #141413;   /* Primary text - deepest but not pure black, warm olive undertone */
--dark-warm:   #3d3d3a;   /* Secondary text, table headers, links */
--olive:       #504e49;   /* Subtext - descriptions, captions. zh-CN TsangerJinKai02 不需要 override. JA override: #4d4c48 (YuMincho thin strokes need darker text) */
--stone:       #6b6a64;   /* Tertiary - dates, metadata */
```

Four levels: near-black (primary) > dark-warm (secondary) > olive (subtext) > stone (tertiary). No fifth level needed.

**Mnemonic**: every gray has a **yellow-brown undertone**. In `rgb()`, warm gray is R ≈ G > B (or R > G > B with small gaps). Cool gray is R < G < B or R = G = B (neutral).

### Border

```css
--border:      #e8e6dc;   /* Primary border - section dividers, table headers, card borders */
--border-soft: #e5e3d8;   /* Secondary border - row separators, subtle dividers */
```

### Semantic warm accent (the one sanctioned exception)

```css
--breaking-bg: #f0e0d8;   /* Changelog .tag.breaking background - muted warm peach */
--breaking-fg: #8b4513;   /* Changelog .tag.breaking text - warm brown */
```

The "no second chromatic color" rule has exactly one approved exception: the breaking-change badge in the changelog template needs a warm warning tint to read as "caution" without importing a red. Both values are warm-toned (R > G > B), registered as `--breaking-*` tokens in `tokens.json`, and enforced by the off-palette lint guard. Any other off-token color is a violation. Do not add a second semantic accent without registering it here and in `tokens.json`.

### Translucent -> Solid conversion (TAGS MUST BE SOLID)

**Why**: WeasyPrint's alpha compositing for padding vs glyph areas produces a visible double rectangle on zoom. See `production.md` Part 4 Pitfall #1.

Ink Blue `#1B365D` over parchment `#f5f4ed` resolves to two registered tokens, and those are the only two the document templates use (the public site keeps a `.tag.brush` gradient in `styles.css` for its own design-system showcase):

| Token | Hex | Use |
|---|---|---|
| `--tag-bg` | `#E4ECF5` | the default tag swatch |
| `--brand-tint` | `#EEF2F7` | the lightest fill, when a tag must recede |

Use the token, never a hand-mixed `rgba()`. A tint outside these two is a new token: add it to `tokens.json` first, or `scripts/tokens.py` will fail the sync guard across the templates that define it.

---

## 2. Typography

### Stacks

```css
/* Single serif per page. --sans always equals var(--serif). */

/* English */
font-family: Charter, Georgia, Palatino,
             "Times New Roman", serif;

/* Chinese */
font-family: "TsangerJinKai02",
             "Source Han Serif SC", "Source Han Serif CN", "Noto Serif CJK SC", "Noto Serif SC",
             "Songti SC", "STSong",
             Georgia, serif;

/* Japanese */
font-family: "YuMincho", "Yu Mincho",
             "Hiragino Mincho ProN",
             "Noto Serif CJK JP", "Source Han Serif JP",
             "TsangerJinKai02",
             Georgia, serif;

/* Mono, with CJK fallback for comments and labels */
font-family: "JetBrains Mono", "SF Mono", "Fira Code",
             Consolas, Monaco,
             "TsangerJinKai02", "Source Han Serif SC",
             monospace;
```

Any font-family that may render Chinese or Japanese must include a CJK fallback, including `@page` footer text, `pre`, `code`, and SVG labels. A pure mono stack can render missing glyph boxes in WeasyPrint.

### Size scale (pt for print A4, px for screen)

**Print:**

| Role | Size | Weight | Line-height | Use |
|---|---|---|---|---|
| Display | 36pt | 500 | 1.10 | Cover title, one-pager hero |
| H1 Section | 22pt | 500 | 1.20 | Chapter titles |
| H2 | 16pt | 500 | 1.25 | Subsection |
| H3 | 13pt | 500 | 1.30 | Item titles |
| Body Lead | 11pt | 400 | 1.55 | Intro paragraphs |
| Body | 10pt | 400 | 1.55 | Reading body |
| Body Dense | 9.2pt | 400 | 1.42 | Dense body (resume, one-pager) |
| Caption | 9pt | 400 | 1.45 | Notes, figure captions |
| Label | 9pt | 600 | 1.35 | Small labels, corner tags |
| Tiny | 9pt | 400 | 1.40 | Footer, minor metadata |

**Screen (px)** ≈ pt × 1.33 (9pt ≈ 12px, 18pt ≈ 24px).
**Minimum floor**: web text >= 12px, PDF text >= 9pt.
**Slide caption floor**: slides 上 caption 至少 24px (不是 12px)。Print 9pt 在投影距离不可读，slide caption 用 pt x 2.67。

**Ladder discipline**: sizes must land ON the scale, never between its steps. Components that each pick their own size drift into 12.5 / 13.5 / 14.5 / 17.5 neighbours; a reader cannot tell a 13.5 from a 14, so the difference registers as noise instead of hierarchy. Audit a screen stylesheet with `grep 'font-size:' <file> | sort | uniq -c`: every value should be a scale step, and the 12px floor should carry only uppercase micro-labels and badges, never prose (prose stops at 14).

**Serif sets larger than sans at the same number**: a tall x-height serif at 15px already reads bigger than 15px sans. Do not raise a serif body size to fix perceived readability; check the ladder step first.

### Weight

- **Serif body**: 400 (W04 font file)
- **Serif headings**: 500 (W05 font file, real bold, not synthetic)
- **Sans body**: 400 default
- **Sans labels / small titles**: 500 or 600
- **Forbidden**: 900 black, 100 thin

**Design principle**: Serif uses only two weights (400/500), no synthetic bold (600/700), maintaining restrained typography.

- `strong { font-weight: 500 }` in long-doc templates locks bold to W05, preventing browsers from synthesizing 700 on top of W05
- **Web only**: W04 covers weight 400-500 (single `font-weight: 400 500` declaration); W05 is PDF-only because WeasyPrint cannot synthesize bold

### Line-height

Print documents are **tighter** than English web body. English web typically runs 1.6-1.75; in print at pt sizes that feels loose and floats.

| Tier | Value | Use |
|---|---|---|
| Tight headline | 1.10-1.30 | Display, H1, H2 |
| Dense body | 1.40-1.45 | Resume, one-pager, dense information |
| Reading body | 1.50-1.55 | Long-doc chapters, letters |
| Label / caption | 1.30-1.40 | Small labels, multi-line metadata |
| CJK screen body | 1.55-1.65 | 中日文 serif 在 slide scale (27-33px) 下比 print x1.33 更需松行高 |

**Forbidden**:
- 1.60+ on a print body - loose feel, web rhythm, not print. The CJK screen-body row above is the one exception, and only at slide scale.
- 1.00-1.05 - lines collide except at extreme display sizes

### Letter-spacing

- Body text: **0**
- Chinese and Japanese body text with TsangerJinKai02: **0.3pt**, the baseline every shipped CN/JA template uses (`long-doc.html`, `one-pager.html`, `slides-weasy.html`); section titles and Mincho samples: **0**
- Chinese lede text (14–22pt) with TsangerJinKai02: **0.03–0.06em** to open up large-body paragraphs without breaking density; EN and JA lede: **0** (only TsangerJinKai02 needs density compensation)
- Chinese and Japanese display text (24pt+): **0.2–1pt** optical spacing for visual breathing room at large sizes; scale with font size
- English headings may use subtle optical tightening when needed; keep it localized, never inherited by body copy
- Small labels (< 10pt): +0.2 to +0.5pt for readability
- All-caps overlines: +0.5 to +1pt mandatory
- **Slide-specific**: print tracking x0.5 at slide scale. Eyebrow max 3px (not 8px), display titles -0.5pt. Large type at 40pt+ will look scattered at print tracking values

---

## 3. Spacing

### Base unit: 4pt (4px on screen)

| Tier | Value | Use |
|---|---|---|
| xs | 2-3pt | Inline adjacent elements |
| sm | 4-5pt | Tag padding, dense layout |
| md | 8-10pt | Component interior |
| lg | 16-20pt | Between components / card padding |
| xl | 24-32pt | Section-title margins |
| 2xl | 40-60pt | Between major sections |
| 3xl | 80-120pt | Between chapters (long docs) |

**Proximity law (a heading belongs to what follows)**: the gap UNDER a section head must be clearly smaller than the gap ABOVE it, ideally by 2x or more. Get this backwards (generous below, tight above) and the head reads as the tail of the previous section, no matter how large its type is. A screen page runs on three steps and nothing else: inside a block, head-to-content, section-to-section.

**A cramped head is an internal problem**: when a heading block feels dense, the cause is usually its own internals (eyebrow-to-title, title-to-lede), not the section gaps. Open those by a few points before touching the macro rhythm.

**Two-column rows: cap both tracks, justify to the edges.** In an image-plus-copy row, let the copy column grow freely (`1fr`) and the text strands itself against an empty far edge. Cap both tracks and push the slack into the gutter instead: both outer edges stay aligned to the container, and the copy sits at its natural measure.

**A repeated row is no place for ornament.** A connector line, a badge, or an index number that looks charming once becomes noise on the fifth repetition. Empty space between the two columns is the answer, not a graphic that fills it.

### Page margins (A4)

| Document | Top | Right | Bottom | Left |
|---|---|---|---|---|
| Resume (dense) | 11mm | 13mm | 11mm | 13mm |
| One-Pager | 15mm | 18mm | 15mm | 18mm |
| Long Doc | 20mm | 22mm | 22mm | 22mm |
| Letter | 25mm | 25mm | 25mm | 25mm |
| Portfolio | 12mm | 15mm | 12mm | 15mm |

**Rule**: denser = smaller margins, more formal (letter) = larger margins.

### Slide-scale spacing

Print uses mm/pt; slides (screen) use px. The scale relationships differ:

```css
--slide-pad: 80px;   /* slide four-side padding baseline */
```

**Key rules**:
- Slide padding-top: 72-80px (print is 96-120px; slides are more compact)
- Macro scale (font size, padding): multiply print pt values by ~1.6
- Micro scale (display tracking, border, radius): halve the print value. Wide display tracking falls apart at slide scale; body letter-spacing stays at the print baseline.

---

## 4. Components

### Cards / Containers

```css
.card {
  background: var(--ivory);
  border-radius: 4pt;
  padding: 16pt 20pt;
}

.card-accent {                              /* when a card must be marked out */
  border-left: 1.4pt solid var(--brand);
}
```

A lifted surface is carried by the fill, not by an outline: `--ivory` against
`--parchment` is the whole gesture. Do not add a closed hairline border. Below
1pt a closed border plus a radius renders as a double ring in WeasyPrint
(`production.md` pitfall #2), and `scripts/lint.py` fails templates for it.
When a card needs more weight than its fill, mark one edge (`equity-report`
`.analyst-box`) rather than ringing all four.

Print radius: 2pt for chips, 4pt for blocks (cards, code, tables). Larger steps
(8pt and up) belong to screen surfaces only, where `landing-page.html` sets its
own scale; on a printed page they read as a web component dropped into a
document.

### The brand left rule

One gesture, three weights. The weight tracks what the rule is doing, not the
size of the type next to it (the 2.5pt tier spans 10pt to 32pt headings):

| Weight | Role | Where it ships |
|---|---|---|
| 2.5pt | Structural divide: a heading that opens a section or document | `changelog` `h2`, `long-doc` `h1` / `.toc h2`, `letter` `.subject`, `portfolio` titles |
| 2pt | Aside: a passage lifted out of the reading flow | `.callout` and `.quote` across one-pager, long-doc, equity-report |
| 1.4pt | Edge of a filled block, where the fill already carries the weight | `equity-report` `.analyst-box`, `long-doc` `.exec-summary` |

Pick the tier by role, then leave the number alone. A fourth value is not a new
idea, it is drift: the same `.callout` sitting at two widths is what teaches a
reader of these templates that the number is theirs to choose. Most documents
need only the 2pt tier; the structural weight is for templates that are scanned
for boundaries rather than read straight through, and heads in a normal document
carry their hierarchy through type alone.

### Buttons

```css
/* Primary - brand-colored */
.btn-primary {
  background: var(--brand);
  color: var(--ivory);
  padding: 8pt 16pt;
  border-radius: 8pt;
  box-shadow: 0 0 0 1pt var(--brand);   /* ring shadow */
}

/* Secondary - warm-sand */
.btn-secondary {
  background: var(--warm-sand);
  color: var(--dark-warm);
  padding: 8pt 16pt;
  border-radius: 8pt;
  box-shadow: 0 0 0 1pt var(--border);
}
```

### Tags

Two tiers, both on registered tokens. A gradient tag oversells itself at this size; if the reader's eye lands on the tag's background shape before the text inside, the tag is too strong.

**Default**:
```css
.tag {
  background: var(--tag-bg);   /* #E4ECF5 */
  color: var(--brand);
  font-size: 9pt;
  font-weight: 600;
  padding: 1pt 6pt;
  border-radius: 4pt;
  letter-spacing: 0.4pt;
  text-transform: uppercase;
}
```

**Recede** (dense pages, or several tags in one row):
```css
.tag {
  background: var(--brand-tint);   /* #EEF2F7 */
  color: var(--brand);
  padding: 1pt 5pt;
  border-radius: 2pt;
}
```

**Philosophy**: tint depth should be one step lighter than what decoration wants. Prefer pale over saturated. In iteration, "gradient brush" often steals focus - lightest solid wins most of the time.

**Never**: `background: rgba(201, 100, 66, 0.18)` - WeasyPrint double-rectangle bug.

### Lists

Use native list markers, brand-colored: ordered lists carry numbers, unordered lists carry a disc. Do not fake a bullet with a `::before` en-dash; a dash marker reads like AI default output, not editorial typesetting. The `ul.dash` class is an alias for the same native rendering, kept only so existing markup keeps working.

```css
ul, ol {
  padding-left: 16pt;
  line-height: 1.55;
}
ul li::marker { color: var(--brand); }
ol li::marker { color: var(--brand); font-weight: 500; }
ul.dash { padding-left: 16pt; }              /* native disc, no en-dash hack */
ul.dash li::marker { color: var(--brand); }
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

### Code

```css
.code-block {
  background: var(--ivory);
  border-radius: 4pt;              /* fill only; no border, see «Cards» */
  padding: 10pt 14pt;
  font-family: var(--mono);
  font-size: 9pt;
  line-height: 1.5;
}
```

### Section Title

```css
.section-title {
  font-family: var(--serif);
  font-size: 14pt;
  font-weight: 500;
  color: var(--near-black);
  margin: 24pt 0 10pt 0;
  border-left: 2.5pt solid var(--brand);
  border-radius: 1.5pt;
  padding-left: 8pt;
}
```

Resume exception: resume templates use a quiet bottom rule instead of the brand left bar, and project rows stay borderless to avoid double rules and page-top orphan lines.

Document header signature: across the document templates (one-pager, changelog, equity-report, long-doc cover) the page header opens with an uppercase eyebrow led by the 8pt x 1.5pt brand tick (a short horizontal bar, not a round bullet, which reads juvenile next to CJK), then the serif title, with any meta right-aligned and a 0.5pt hairline rule closing the block. This is the shared opener: no centered version block, no full-height left bar. The full-height brand left bar reads heavy and crude as a page-top frame, so keep it a section-level and pull-quote device only. Resume (name header) and letter (letterhead) keep their purpose-built headers and are exempt.

Hero product shot (one-pager): a product brief earns one real screenshot as its visual anchor, not a decorative texture. Frame it in a wrapper with `overflow: hidden`, `border-radius`, and a soft shadow (no closed sub-1pt border, which trips the `thin-border-radius` lint and risks a double ring); size the wrapper by `height` with the image set to `object-fit: cover` so dead background (wallpaper, chrome margins) is trimmed evenly while the app window stays whole. Give it a single caption that adds a fact, not a restatement. Adjust the wrapper height to fill the page rather than stretching text or leaving bottom whitespace.

Changelog best practice: a release-notes doc uses the same editorial language as the one-pager, not a centered version block. Open with the left-aligned header (eyebrow tick + "Project Version" serif title + date on the right + hairline rule). Group entries under h2 section heads carrying the brand left bar (Breaking / Features / Fixes, or Highlights / Fixes), and drop any section that does not apply. Each entry is a numbered item with a bold lead-in, then a colon and the detail; the numbers carry sequence and restart per section, so no per-item bullet glyph is added (one repeated mark per row reads as clutter). Keep acknowledgements a quiet labelled note, never a filled card, and split the footer into left description and right URL. One locale per file, no bilingual stacking. A breaking entry may carry an inline `.tag.breaking` chip; that is the only inline tag worth keeping.

### Table (kami-table)

Unified table component across all templates. Base class applies to bare `<table>` or `.kami-table`.

```css
table, .kami-table {
  width: 100%; border-collapse: collapse;
  font-size: 9.5pt; margin: 12pt 0; break-inside: avoid;
}
table th, .kami-table th {
  text-align: left; font-weight: 500; color: var(--dark-warm);
  padding: 6pt 8pt; border-bottom: 1pt solid var(--border);
}
table td, .kami-table td {
  padding: 5pt 8pt; border-bottom: 0.3pt solid var(--border-soft);
  vertical-align: top;
}
```

**Variants** (combine freely on the same element):

| Class | Purpose |
|---|---|
| `.compact` | 8pt font, tighter padding. For data-dense tables in resume/one-pager. |
| `.financial` | Right-align all columns except the first, enable `tabular-nums`. For revenue, pricing, metrics. |
| `.striped` | Alternating `var(--ivory)` background on even rows. Improves scanability for wide tables. |

**Total row**: add `.total` to the final `<tr>` for a bold summary row with a `1pt` brand top border.

```html
<table class="kami-table financial striped">
  <thead><tr><th>Category</th><th>Q1</th><th>Q2</th></tr></thead>
  <tbody>
    <tr><td>Revenue</td><td>$12.4M</td><td>$14.1M</td></tr>
    <tr class="total"><td>Total</td><td>$12.4M</td><td>$14.1M</td></tr>
  </tbody>
</table>
```

### Metric

Key numbers side-by-side (one-pager header, resume top, portfolio cover):

```css
.metrics { display: flex; gap: 24pt; }
.metric  { flex: 1; display: flex; align-items: baseline; gap: 6pt; }
.metric-value {
  font-family: var(--serif);
  font-size: 16pt;
  font-weight: 500;
  color: var(--brand);
  font-variant-numeric: tabular-nums;   /* align digits in columns */
}
.metric-label { font-size: 9pt; color: var(--olive); white-space: nowrap; }
```

This inline form is the print one, and it holds only because print labels are fixed short strings that never wrap. The value and label share a baseline; a label that wraps to a second line dangles below it and reads broken. Keep every label short enough for one line and set `white-space: nowrap`, so an over-long label surfaces as overflow during QA instead of silently wrapping. Fix by shortening the words, not by letting it wrap.

On screen the labels are translated, retitled, and read at 375px, so a landing-page metric stacks instead (`flex-direction: column`). Same for slides. `production.md` pitfall #20 owns that call.

### Section Header (`.kami-section-header`)

Lightweight section opener for content slides. Has an eyebrow and a horizontal rule.

```css
.kami-section-header {
  margin-bottom: 36px;
}
.kami-section-header .eyebrow {
  display: flex;
  align-items: center;             /* dot is geometric, center beats baseline */
  gap: 8px;
  font-family: var(--sans);
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 1.5px;
  text-transform: uppercase;
  color: var(--stone);
  margin-bottom: 14px;
}
.kami-section-header .eyebrow::before {
  content: "";
  display: inline-block;
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--brand);
  flex-shrink: 0;
}
.kami-section-header .rule {
  height: 1px;
  background: var(--border);
  margin-bottom: 36px;             /* gap below rule >= 36px (>= 2x the gap above) */
}
.kami-section-header h1 {
  font-family: var(--serif);
  font-size: 38px;
  font-weight: 500;
  line-height: 1.1;
  color: var(--near-black);
}
```

**Spacing rule**: eyebrow to rule: 14px; rule to H1: **≥ 36px** (the gap below must be at least double the gap above, creating a visual anchor).

### Code Card (`.kami-code-card`)

For displaying pseudocode or code snippets in slides. More structured than a plain code block.

```css
.kami-code-card {
  background: var(--ivory);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px 24px;
  overflow: hidden;
}
.kami-code-card pre {
  font-family: var(--mono);
  font-size: 13px;                 /* 14px for larger slides */
  line-height: 1.55;
  color: var(--near-black);
  margin: 0;
  white-space: pre;
}
/* Syntax colors: existing tokens only, no new colors */
.kami-code-card .k { color: var(--brand); }    /* keyword / string */
.kami-code-card .c { color: var(--stone); }    /* comment */

/* Optional line numbers: 1px left divider */
.kami-code-card.numbered {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0 16px;
}
.kami-code-card .line-nums {
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.55;
  color: var(--stone);
  text-align: right;
  border-right: 1px solid var(--border-soft);
  padding-right: 12px;
  user-select: none;
}
```

**Content philosophy**: use pseudocode style. Comments should outnumber code lines. The reader sees logic, not syntax.

### Syntax Highlighting

Code blocks with `class="language-*"` on the `<code>` element get Pygments-based highlighting at build time. The palette uses existing tokens only:

| Token | Hex | Token var |
|---|---|---|
| Keyword | `#1B365D` | `--brand` |
| Comment | `#6b6a64` | `--stone` |
| String | `#504e49` | `--olive` |
| Number | `#3d3d3a` | `--dark-warm` |
| Function/Class | `#141413` | `--near-black` |

```html
<pre><code class="language-python">def analyze(data):
    """Transform raw data."""
    return transform(data)</code></pre>
```

Blocks without `class="language-*"` stay monochrome. Requires `pip install Pygments`; without it, blocks pass through unstyled.

### Glance Grid

Four key-number cells, placed after the TOC or on a chapter-opening page of a long-doc / proposal.

```html
<div class="glance-grid">
  <div class="glance-cell">
    <div class="glance-label">REPORTING PERIOD</div>
    <div class="glance-value">Q1 2026</div>
    <div class="glance-note">Three core themes</div>
  </div>
  <!-- 4 cells total -->
</div>
```

```css
.glance-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14pt;
  margin: 18pt 0;
}
.glance-cell {
  padding: 12pt 0 10pt 14pt;
  border-left: 2pt solid var(--brand);
  border-radius: 1.5pt;
}
.glance-label {
  font-family: var(--mono);
  font-size: 8.5pt;
  color: var(--brand);
  letter-spacing: 1pt;
  text-transform: uppercase;
  font-weight: 500;
}
.glance-value {
  font-size: 18pt;
  font-weight: 500;
  color: var(--near-black);
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.5pt;
}
.glance-note {
  font-size: 9pt;
  color: var(--olive);
  line-height: 1.4;
}
```

### Module Block

Proposal A / B / C structure: each module gets a brand-colored letter, a Chinese title, and an uppercase English subtitle.

```html
<div class="module">
  <div class="module-head">
    <div class="module-letter">A</div>
    <div class="module-title">模块标题</div>
    <div class="module-sub">MODULE SUBTITLE</div>
  </div>
  <p>...</p>
  <ul>...</ul>
</div>
```

Visual recipe: letter at 28pt brand, title at 17pt, English subtitle at 10pt mono brand `letter-spacing: 2pt`, head separated from body by a 0.3pt warm-gray hairline (not brand color).

### Module Note (group explanation)

A short note that explains the relationship between two or more modules. Same family as `.callout`, lighter weight, no decorative bar.

```html
<div class="module-note">
  <div class="module-note-label">ABOUT B + C</div>
  <p>B 是上游能力建设，C 是下游验证。两者构成一个最小闭环。</p>
</div>
```

ivory background + 4pt radius + `module-note-label` at 8.5pt brand uppercase mono.

### Position Table

Three-column industry-comparison table whose final row highlights the current project / subject.

```html
<table class="position-table">
  <thead>
    <tr><th>Direction</th><th>Reference project</th><th>Approach</th></tr>
  </thead>
  <tbody>
    <tr><td>...</td><td>...</td><td>...</td></tr>
    <tr class="highlight"><td><strong>本项目</strong></td>...</tr>
  </tbody>
</table>
```

`.highlight` row: ivory fill + brand text. Do not bold the entire row; let the `<strong>` carry the emphasis.

### Pricing Card

Headline-figure price block. Eyebrow + price + short note.

```html
<div class="pricing-card">
  <div class="pricing-eyebrow">PROJECT TERM</div>
  <div class="pricing-price">
    <span class="currency">¥</span>
    <span class="amount">XXX,XXX</span>
    <span class="unit">/ term</span>
  </div>
  <div class="pricing-note">Paid by milestone</div>
</div>
```

Digits: serif 500, 44pt, `tabular-nums`, `letter-spacing: 0.5pt`. Without the letter-spacing, large digits crowd each other and read as too dense.

For the without-price variant (see writing.md "Proposal voice"), drop the `.pricing-price` block and follow the eyebrow with the Value Anchors list below.

### Value Anchors

Replaces pricing line-item breakdowns with a short list of capability anchors. Pairs with Pricing Card or stands alone in the without-price variant.

```html
<ul class="value-anchors">
  <li><strong>能力锚点 A</strong>：一句具体说明能力来源的事实陈述</li>
  <li><strong>能力锚点 B</strong>：一句具体说明锚点稀缺性的事实陈述</li>
  <!-- 4-6 items -->
</ul>
```

```css
.value-anchors {
  list-style: none;
  padding: 0;
  margin: 12pt 0 18pt 0;
}
.value-anchors li {
  position: relative;
  padding: 9pt 0 9pt 18pt;
  border-bottom: 0.3pt solid var(--border-soft);
  line-height: 1.55;
  font-size: 10.5pt;
}
.value-anchors li:last-child { border-bottom: none; }
.value-anchors li::before {
  content: "";
  position: absolute;
  left: 0;
  top: 17pt;
  width: 8pt;
  height: 1.5pt;
  background: var(--brand);
}
.value-anchors li strong {
  color: var(--brand);
  font-weight: 500;
  margin-right: 6pt;
}
```

The 8pt × 1.5pt brand bar (`::before`) replaces the round `<ul>` bullet. A round bullet next to CJK body reads juvenile; the bar reads editorial.

### Decoration density: editorial vs structured

Long-doc / proposal layouts have two acceptable decoration densities. Pick one and stay consistent across the whole document.

| Context | Mode | Pattern |
|---|---|---|
| Data report, white paper, technical brief | **Structured** | Top hairlines (0.6-0.8pt brand) on callouts, glance cells, and pricing blocks. Roughly 5-8 brand lines per page. |
| Proposal, advisory pitch, founder-facing brief | **Editorial** (default) | No decorative lines. Brand color appears only in text (chapter number, `.hl`, `<strong>`, digits, labels). Containers use ivory fill + 4pt radius. |

The editorial mode reads as "content speaks"; the structured mode reads as "structure helps". The wrong mode is the third one: brand lines plus ivory plus radius plus borders, which signals over-packaging. When unsure, default to editorial.

---

## 5. Depth & Shadow

**Core rule**: do not use traditional hard shadows. Depth comes from three sources:

### 1. Ring shadow (border-like)

For **button** hover/focus states.

```css
/* Button default */
box-shadow: 0 0 0 1px var(--border);

/* Button hover/active */
box-shadow: 0 0 0 1px var(--brand);
```

**Do not use for card hover**: ring shadow is a border replacement. Layering it over an existing border creates three-layer visual stacking (border + ring + offset), which feels digital, not paper-like.

### 2. Whisper shadow (barely visible lift)

For **card hover** and **featured card** elevation.

```css
/* Card hover - mimics paper lifting slightly */
.card {
  transition: box-shadow 0.2s;
}
.card:hover {
  box-shadow: 0 4pt 24pt rgba(0, 0, 0, 0.05);
}

/* Featured card default state */
.featured-card {
  box-shadow: 0 4pt 24pt rgba(0, 0, 0, 0.05);
}
```

**Why whisper, not ring**: paper elevation is depth change, not outline change. Whisper shadow is singular, soft, outline-free, matching the paper-like tone.

### 3. Section-level light/dark alternation

Long docs alternate parchment `#f5f4ed` and `#141413` dark sections. This section-level light change creates the strongest contrast.

**Forbidden**: `box-shadow: 0 2px 8px rgba(0,0,0,0.3)` and relatives.

---

## 6. Print & Pagination

### break-inside protection

```css
.card, .metric, .project-item, .quote, .code-block, figure, .callout,
.takeaway, .module, .module-note, .glance-grid, .pricing-card,
table.compact {
  break-inside: avoid;
}

/* Headings should never sit alone at the bottom of a page */
h1, h2, h3 { break-after: avoid; }

/* Widow / orphan minimums for body text */
body { widows: 3; orphans: 3; }
p    { widows: 2; orphans: 2; }
```

CSS alone cannot prevent "the last two lines of a chapter pushed onto a fresh page". For long-doc / proposal output, follow up with a render-time density check (see production.md "Verify & Debug").

Long-doc table-of-contents rows should link to stable chapter ids and use
WeasyPrint `target-counter(attr(href), page)` for rendered page numbers. Do not
hand-fill page numerals; any pagination-affecting edit will make them drift.

**Cascading break-inside**: when two `break-inside: avoid` blocks sit next to each other and the first would split, both get pushed to the next page together. A chapter with more than two `break-inside: avoid` blocks (quote + table + callout, etc.) near a page boundary is at high risk of leaving 40-80mm of trailing whitespace on the previous page. Fix by splitting the chapter, or downgrade one block (allow the table to break with a repeating header `<thead>`).

### Force break

```css
.page-break { break-before: page; }
```

### Page background extending past margins

```css
@page {
  size: A4;
  margin: 20mm 22mm;
  background: #f5f4ed;   /* extends past margin area, prevents printed white edges */
}
```

---

## 7. Quick decisions

When you're not sure "what should I use":

The answer is almost always a component a template already ships. Copy that one
and edit its content; assembling a new container from a recipe is how a document
ends up carrying three unrelated emphasis languages on one page.

| Need | Use |
|---|---|
| Big headline | serif 500, size by level, line-height 1.10-1.30 |
| Reading body | serif 400, 9.5-10pt, line-height 1.55. Every locale: CN templates pin `--sans: var(--serif)`, so one page carries one typeface |
| Emphasize a number | `color: var(--brand)`, no bold |
| Raise a passage above body text | `.callout`: ivory fill + 2pt brand left rule + 3pt radius. Identical in one-pager, long-doc and equity-report; only padding tightens on denser pages. One emphasis form per page, reused |
| Quote someone | `long-doc` `blockquote` / `.quote`: same 2pt left rule, olive text, but no fill. The rule is shared; the fill is what separates a quotation from a raised passage |
| Show code | `long-doc` `pre` / `code`: ivory fill, 4pt / 2pt radius, no border |
| Show key figures | `one-pager` `.metric`: baseline row, transparent, no container. Numbers carry themselves; a filled card around them is the most common drift |
| Start a section | `long-doc` `h2`: serif, no left bar. `changelog` `h2` carries the bar because release notes need scannable group heads, and it is the exception |
| Mark out one item in a list | one edge, `border-left: 1.4pt solid var(--brand)` (`equity-report` `.analyst-box`) |
| Cover page | `long-doc` cover: display heading, right-aligned author/date, heavy whitespace |
| Buttons (screen only) | `landing-page` `.btn-primary` / `.btn-ghost`. Print documents have no buttons |

Nothing here fits -> return to first principles: **serif carries authority,
sans carries utility, warm gray carries rhythm, ink-blue carries focus**. Then
add the smallest thing that works, and prefer an existing class over a new one.

---

## 8. Deck Recipe

Slides in kami use WeasyPrint HTML to PDF as the primary rendering path. The pptx path (`slides.py`) is available as a fallback when the user explicitly requires an editable PPTX file.

### Architecture

**Why WeasyPrint over python-pptx:** pptx output passed through LibreOffice loses CJK font weight, tracking, and glyph spacing. WeasyPrint embeds fonts exactly, giving pixel-level CSS control.

Use `assets/templates/slides-weasy.html` (CN) or `assets/templates/slides-weasy-en.html` (EN) as the starting point.

### Page size

Default `280mm 158mm`. Change in `@page` and `.slide` together.

| Size | `@page` | Use when |
|---|---|---|
| Compact (default) | `280mm 158mm` | Standard density, fits most content |
| Standard | `297mm 167mm` | Slightly more room per slide |
| Wide | `338mm 190mm` | Heavy content, many data points |

### Typography

Global parameters for the slide body:

```css
body {
  font-size: 13pt;
  line-height: 1.55;
  letter-spacing: 0.3pt;   /* CJK: critical for breathing room */
}
```

Heading scale:

| Element | Size | Weight | Notes |
|---|---|---|---|
| `h2` | 24pt | 500 | Page title; `margin-bottom: 14pt` |
| `h3` | 15pt | 500 | Section heading; `color: var(--brand)` |
| `.eyebrow` | 9.5pt | 400 | Mono, `letter-spacing: 2pt`, `color: var(--stone)` |
| `.lead` | 12pt | 400 | Below `h2`; `color: var(--olive)` |

Content element scale:

| Class | Size | Notes |
|---|---|---|
| `.mt` | 16pt | Module title, used with `.ml` |
| `.ml` | 24pt | Large letter prefix in `var(--brand)`, paired with `.mt` |
| `.ms` | 7.5pt mono | Module sub-label; `border-bottom` separator |
| `.mb` | 11pt | Module body description |
| `.mi` | 11pt | Module line item; `padding: 8pt 0` |
| `.mc` | 9.5pt | Delivery rhythm or cadence note; `border-top` |
| `.co` | 11pt bold | Bottom callout; `position: absolute; bottom: 12mm` |

### Layout patterns

**Two-column (`.c2`)**: CSS Grid, `grid-template-columns: 1fr 1fr; gap: 22pt`. Use for side-by-side modules with independent heights.

**2×2 aligned (`.t2x2`)**: HTML `<table>`, not CSS Grid. Grid does not guarantee row alignment across cells; table rows share height naturally.

```html
<table class="t2x2">
  <tr>
    <td> <!-- top-left --> </td>
    <td> <!-- top-right --> </td>
  </tr>
  <tr>
    <td> <!-- bottom-left --> </td>
    <td> <!-- bottom-right --> </td>
  </tr>
</table>
```

**Pinned callout (`.co`)**: `position: absolute; bottom: 12mm; left: 20mm; right: 20mm`. The whitespace above it is intentional, not empty.

### Table styles

```css
table.data td {
  padding: 8pt;
  border-bottom: 0.3pt solid var(--border);
  font-size: 11pt;
}
table.data td:first-child {
  font-weight: 500;
  color: var(--brand);   /* first column: brand blue bold */
}
```

### SVG constraints

- `viewBox` width fixed at `920`; adjust height to content
- `max-height: 105mm` on `svg` element to prevent overflow
- WeasyPrint does not support `fill="url(#gradient)"` or CSS Grid inside SVG
- Draw arrowheads as explicit `<path>` elements; `marker-end` with `orient="auto"` does not rotate in WeasyPrint

### Content rules

| Rule | Detail |
|---|---|
| No section divider slides | Use `.eyebrow` for section numbering instead; saves one slide per section |
| No CJK parentheses | Replace `（...）` with `·` or `,` |
| Ghost deck test | Read only slide titles in order. They must tell the argument; disconnected titles mean the structure is not ready |
| One evidence shape | Each slide has one primary proof form: chart, table, screenshot, code, quote, or conclusion. Split mixed evidence |
| One line per bullet | Trim until each item fits on one line; never let it wrap |
| Empty space ≥50% | Draft defect. Order: merge with neighbor slide > pin `.co` callout > add a chart that earns the space. Shrinking page size is a last resort and must apply to the whole deck, not per slide. |
| Empty space 25-50% | Acceptable if the slide has a pinned `.co` callout. Otherwise add one supporting bullet or a small inline figure. Never pad with filler prose. |
| Cover | No horizontal rule; title centered `38pt`; subtitle on one line; bottom meta centered |

Image-heavy decks carry two acceptance bars: the visual brief (crop notes, prompt fragments, generation instructions) is internal working material and never appears in slide titles, body copy, or captions; and the deck uses the existing `.c2`, `table.t2x2`, `.co`, data table, and inline figure patterns unless the source material clearly needs something else. How you plan toward that (slot map, outline, or otherwise) is your call; whatever you sketch is a rhythm check, not a locked layout registry.

If the user provides a real PPTX or brand template and explicitly asks to preserve it, do a template inventory before content editing: thumbnail the source deck, identify reusable layout families, then map each section to an existing layout. Do not do this for the default WeasyPrint or Marp paths; Kami templates are already the inventory.

### Troubleshooting

| Symptom | Fix |
|---|---|
| Content overflows to next page | Add `max-height` or trim content |
| 2×2 columns misaligned | Switch from CSS Grid to `table.t2x2` |
| Large blank at slide bottom | First check item count (target 3-5 items per slide). If content is genuinely short, pin a `.co` callout. Only reduce page size when the entire deck is uniformly sparse. |
| CJK text looks tight | Add `letter-spacing: 0.3pt` |

### Core principles

1. `letter-spacing` matters more than `font-size` for CJK density
2. No white card panels on parchment; use border lines to divide

### Marp variant

Marp is an optional third path, alongside WeasyPrint HTML and python-pptx. Use it only when the user explicitly asks for Marp, "markdown slides", or a deck that lives in a `.md` file. The repo does not bundle `marp-cli`; rendering happens with the user's local install.

Files:

| Asset | Path |
|---|---|
| CN theme CSS | `assets/templates/marp/slides-marp.css` |
| EN theme CSS | `assets/templates/marp/slides-marp-en.css` |
| CN sample deck | `assets/templates/marp/slides-marp.md` |
| EN sample deck | `assets/templates/marp/slides-marp-en.md` |

Shared with WeasyPrint slides: every design token (`--parchment`, `--brand`, `--serif`, `--mono`), the Kami class scale (`.eyebrow`, `.lead`, `.mt`, `.ml`, `.mb`, `.mc`, `.co`, `.c2`, `table.t2x2`, `table.data`, `section.cover`), and the 280×158mm page size. The Marp theme is a port, not a redesign.

Marp-specific additions on top of that port: the theme styles bare `<p>`, `<ul>`, `<ol>`, `<li>` so that plain Markdown body content picks up Kami rhythm without explicit class attributes. These rules do not exist in `slides-weasy.html` because the WeasyPrint deck never has unclassed Markdown. They are required here because Marpit's defaults would otherwise leak through.

`.co` is pinned at `bottom: 18mm` in the Marp theme, not `12mm` like in `slides-weasy.html`. Reason: the WeasyPrint deck's footer is two narrow corner labels (`.page-num` right, `.footer-mark` left) that never sit under a centered `.co`. Marp's built-in footer spans the full width at `bottom: 10mm`, so `.co` needs a wider vertical buffer to avoid stacking on top of it.

Brand color and logo follow the same `brand-profile.md` Layer C rules: edit `--brand` in the theme CSS; insert a logo with `<img src="../../images/logo.svg" width="80">` on the cover slide. The output-path caveat in `production.md` Part 2.5 applies to logo URLs the same way it applies to fonts.

Marp-specific syntax to know:

| Need | Marp syntax |
|---|---|
| Page break | One blank line, then `---`, then one blank line |
| Per-slide class (e.g. cover) | `<!-- _class: cover -->` at the top of the section |
| Per-slide pagination off | `<!-- _paginate: false -->` (use it on the cover and the closing slide) |
| Per-slide footer override | `<!-- _footer: "..." -->` |
| Global header / footer / pagination | Set in the deck's YAML front-matter (`paginate: true`, `footer: "Project"`) |
| Background image | `![bg]\(path.jpg\)` |

Constraints that bite if you arrive from the WeasyPrint slides:

- The page unit is `section`, not `.slide`. CSS that targets `.slide` will not match. The theme already declares `section { width: 280mm; height: 158mm; position: relative; }`, so the theme's `.co { position: absolute; bottom: 18mm }` still pins to the bottom of the current slide.
- Markdown blocks inside `<div>` wrappers need surrounding blank lines for Marp to parse them as Markdown. The sample deck shows the pattern for `.c2` and `table.t2x2`.
- `paginate: true` injects a page number via the `section::after` pseudo-element. Do not also place a `.page-num` element by hand; you will get two numbers.

Render commands and CLI flags live in `references/production.md` Part 2.5.

---

## 9. Horizontal Funnel / Progress Bar Pattern

No dedicated `funnel.html` diagram prototype exists. When generating a horizontal bar chart with external percentage labels (conversion funnel, progress tracker, ranked list), use a three-column grid so the label column is fixed-width and never drifts with bar length.

```css
.funnel-row {
  display: grid;
  grid-template-columns: 80pt 1fr 40pt;  /* label | track | external % */
  align-items: center;
  gap: 8pt;
  margin: 4pt 0;
}
.funnel-track {
  position: relative;
  height: 18pt;
  background: var(--border-soft);
  border-radius: 2pt;
}
.funnel-fill {
  position: absolute;
  inset: 0 auto 0 0;
  background: var(--brand);
  border-radius: 2pt;
}
.funnel-pct {
  font-variant-numeric: tabular-nums;
  text-align: right;
  color: var(--stone);
  font-size: 9pt;
}
```

Example row (77.8% fill):

```html
<div class="funnel-row">
  <span>Stage Name</span>
  <div class="funnel-track">
    <div class="funnel-fill" style="width:77.8%"></div>
  </div>
  <span class="funnel-pct">77.8%</span>
</div>
```

Key rules:

- Use the three-column grid, not flex. The third column is always 40pt wide; the percentage never moves regardless of bar length.
- Color: single `--brand` fill only. No color gradients or per-row hues. Vary opacity (e.g. `opacity: 0.7`) if a visual ranking is needed, not hue.
- Inline labels inside the bar (count, name) go in an absolutely positioned child inside `.funnel-track`, not in the grid's third column.

---

## 10. Image Aspect Ratios and Cropping

Use this table when placing images in any Kami template. Pick the content slot first, then decide whether the source should be preserved, padded, cropped, or regenerated. The ratios are defaults, not constraints; adjust by one step if the source image differs significantly.

| Context | Preferred ratio | Notes |
|---|---|---|
| Hero full-bleed (slides / cover) | 16:9 | One image fills the slide; use `object-fit: cover` |
| Main document image (long-doc / portfolio spread) | 4:3 or 16:10 | Standard editorial proportion |
| Side-by-side grid (two images per row) | 3:2 | Even weight, comfortable scanning |
| Magazine inset (text wraps around) | 3:2 or 3:4 | Portrait works when the subject is a person |
| Square thumbnail (icon grid, avatar, logo) | 1:1 | Enforced with `aspect-ratio: 1/1` |
| Slide image grid (26vh fixed-height row) | Fixed height, free width | Grid items share a row height; clip width to fit |

**Slot-first rule**: do not generate an image and then hunt for a place to put it. Decide the image job first: proof screenshot, product surface, person/place photo, diagram, logo, or decorative texture. Proof screenshots and product UI keep fidelity; only diagrams and concept images may be redrawn for style.

**Audience copy vs visual brief**: visible copy says what the reader should believe. A visual brief says what the image should contain, crop, preserve, or avoid. Keep the brief in the layout note, comments, or temporary slot map. Never paste prompt fragments such as "16:9 cinematic UI mockup" or crop instructions into captions, bullets, alt text, FAQ, or metadata.

**Screenshot handling**: product screenshots default to `object-fit: contain` inside a stable frame, or to a programmatic canvas with quiet padding when the target ratio differs. Do not crop away UI text, numbers, window chrome, terminal prompts, or controls just to fill a frame. If the screenshot is too tall, too narrow, or too dense, split it into 2-3 panels before considering an AI redraw. When a redraw is unavoidable, label it as a schematic or concept image, not a real screenshot.

**Cropping rule**: choose `object-position` by subject, not as a universal default. UI screenshots use `contain` or centered padding. Photos of people or products use `cover` with the subject in the safe center area (`center center` or `center 35%`). Document scans and pages often prefer `top center` because titles live at the top.

```css
.frame-img,
figure img,
.hero-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center center;
}
```

Apply this to fixed-size containers that intentionally crop photos or illustrations. For `object-fit: contain` (screenshots, slides, logos), `object-position` has little visible effect; use frame padding and alignment instead.

### Brand logo slot

`one-pager`, `portfolio`, and `slides-weasy` (and their `-en` variants) carry an optional brand logo slot, supplied by the brand profile `logo` field (see `references/brand-profile.md` Layer C). It ships commented out, so the default render is unchanged. The `.brand-logo` rule is fixed-height, `width: auto`, `object-fit: contain`, so any aspect ratio scales cleanly without distortion. Sizes are deliberately per template, not a shared token:

| Template | Height | Placement |
|---|---|---|
| one-pager | `44px` | top of the header flex row, `align-self: flex-start` |
| portfolio | `72px` | above the cover eyebrow, inside `.cover-head` |
| slides-weasy | `72px` | centered above the cover `h1` |

Keep the base and `-en` `.brand-logo` rules identical; the cross-template lint pair check (`scripts/lint.py`) flags drift. Do not add `object-position` (no effect under `object-fit: contain`).

---

## 11. Landing Page (screen-first)

The landing-page template is the only kami template designed for browser delivery, not PDF. It inherits the full parchment design system but adds interactive and responsive patterns.

### Product site system

- Use `landing-page*.html` for a single ready-to-serve product page. Keep CSS and JS inline so the file can be copied without a build step.
- If the deliverable needs docs, help, releases, changelog, roadmap, legal pages, or more than two locales, treat it as a production product site.
- For a production product site, prefer one structural template, locale string files, and long-content files. The generator must have a check mode that fails on missing keys and generated-output drift.
- Product positioning must be checked against current product surfaces before rewriting. Stale category language is worse than a missing feature detail.
- Locale pages, FAQ, JSON-LD, `llms.txt`, `llms-full.txt`, screenshots, install copy, pricing, version, and support links are one public fact set. Keep the factual claims aligned across them.
- Do not promote project-specific release artifacts, appcast rules, payment providers, or private local paths into the generic template.

### Product screenshot slots

Use a small slot matrix before filling a landing page or product site. This keeps the default beautiful without adding a new template system:

| Slot | Preferred asset | Fit |
|---|---|---|
| Hero signal | real product surface, terminal state, or app window | 16:10 or 16:9, preserve recognition over full-bleed drama |
| Gallery panel | one shipped workflow or reachable UI state | stable frame, real screenshot first, no unrelated filler |
| Feature panel | focused crop or two-step before/after proof | preserve labels and controls; split dense screenshots |
| Social image | product name + one recognizable surface | 1200x630 crop, repo path or public URL |

Every screenshot path must resolve from the repo or a stable public URL. Never reference `/Users`, `file://`, or sibling checkouts. Missing visuals remain material gaps or omitted panels; they are not replaced with stock atmosphere.

- **Localized UI needs localized captures.** When the product ships a localized UI, each locale page carries captures in that page's language (menu bars, window chrome, gallery states). One shared English screenshot across all locale pages breaks the promise the localized copy just made. Template the image directory per locale so every capture swaps at once, rather than overriding images one by one.
- **Social image bytes must match the declared type.** A PNG renamed `.jpg` ships RGBA bytes against an `image/jpeg` Content-Type and social validators flag or drop it. Encode the og:image as a real baseline JPEG at 1200x630; the re-encode typically also cuts the file to a fraction of the PNG size.

### Layout

- `max-width: 1120px` centered, padding `88px 64px 120px`
- Sections numbered `00 · Label` through `04 · Label` with `section-num` / `section-title` / `section-lede` pattern
- Two responsive breakpoints: `880px` (tablet) and `480px` (phone)
- Section rhythm is a system, not per-gap. Run section spacing as one responsive ladder (e.g. desktop 96/72, tablet 72/54, phone 56/42). When a page reads too airy or too tight, scale the *whole* set by a single factor (about 0.75) across all breakpoints at once; nudging one gap leaves asymmetry, and asymmetry that survives tuning is structural. At the phone breakpoint step gutters down (64px to 16px) and shrink display sizes (hero title, price amount) in the same pass.

### Single-line surfaces

Certain copy surfaces must render as one line; a wrap there reads as a defect, not as text flow. The near-wrap state (one word from wrapping) counts as a failure too, because any locale or font fallback pushes it over.

- Single-line at the desktop baseline (1280px): hero tagline, feature subtitles, benefit points, gallery captions, section ledes under ~12 words, footer ethos.
- Single-line at 375px as well: key-fact tokens (price line, platform line, CTA labels, hero chips).
- Fix order is fixed: cut words first, rephrase second, adjust layout last. Never shrink the font, never force it with `<br>`, never shave padding to buy one word of width.
- Any component whose height depends on its text (carousel captions, rotating taglines) must be verified with the longest shipped locale; a wrap that appears in one locale makes the component jump between slides.
- One wrap found means sweeping every surface in this list across every locale, not fixing the reported spot (see `AGENTS.md` «Critical Line-Break Scan» for the PDF-side counterpart).

### Decorative layers

Backgrounds, particles, connecting lines, and gradient motion get a two-round iteration budget. If the layer still reads as murky, busy, or foreign after the second adjustment round, stop tuning parameters and present a plain-surface version (page background plus the existing typography) next to the current state. The plain version wins far more often than a third round of tuning; decoration that needs three rounds of defense is decoration the page does not need. This is the page-level twin of the feature-row lesson below: when rescuing a gesture keeps failing, the gesture is wrong, not the parameters.

### Mobile density inversion

Below the phone breakpoint, the information diet reverses: images first, words second.

- Feature blocks lead with the image; copy compresses to 1-2 sentences per block.
- FAQ collapses by default (details/summary or equivalent); a phone reader scrolls past 8 open answers and loses the pricing section below them.
- Wrap every hover style in `@media (hover: hover)`. On touch screens a tap must not flash the hover state; a flashing hover color on tap is a shipped bug, not a nicety.

### Eyebrow

- Flex row: `space-between`, left side = product category text + version link, right side = hero-links (lang switch, social icons)
- Version link: brand color, weight 600, clickable to releases page. Slot: `{{VERSION}}`
- `--latin-ui` 12px, weight 500, letter-spacing 0.4px, uppercase, stone color

### Hero

- Title: 96px (EN) / 88px (CN), weight 500, letter-spacing 0
- Entrance animation: `translateY(10px) + blur(6px)` fading in over 900ms with 120ms delay
- Tagline: 21px (EN) / 20px (CN), olive color, letter-spacing 0.2px (EN) / 0.4px (CN), max-width 820px
- Tokens row: a few small chips as `<span>quality</span>`, 13px stone, `--latin-ui` font
- CTA: pill buttons (border-radius 999px), primary filled + ghost outlined, 15px, 13px 28px padding
- Quality chips, not a facts list. The tokens row should carry product *qualities* (good-looking, lightweight, AI-friendly), not an inventory (license, package manager, OS version). Push every hard fact to the footer or docs where it is referenceable. Pick about three.
- No chip may repeat the tagline. Read tagline and chips together and cut any concept stated twice. If trimming a chip leaves an orphaned separator, the row should collapse to one clean line, not a dangling dot.
- Wrap-safe chip separator. Put the middot on `span:not(:last-child)::after`, never on `::before` of the following item, so a chip that wraps to the next line never carries a leading dot. Use `color-mix(... 58%, transparent)` so the dot stays quieter than the text.
- Line-widow discipline (title + tagline). Eliminate 1-2 word last lines by trimming the copy so the block rebalances, not by adding a `max-width` cap (a cap narrower than its container wraps early and leaves empty space on the right, which reads as a premature break). `text-wrap: balance` on the title and `pretty` on the tagline help only as a backstop; do not rely on them. Leave inherently-two-line notes alone.

### Gallery

- Grid: `minmax(0, 1fr) auto`, frame spans full width, caption and tabs on row 2
- Frame: dark background `--shot-bg: #141318`, rounded 8px, 1px border
- Screenshots are final product surfaces first. Use real app/site captures over mockups; if the asset is missing, record a material gap or omit that panel rather than substituting unrelated imagery.
- Transition: direction-aware slide + scale(0.985), 620-880ms cubic-bezier(0.22, 1, 0.36, 1)
- Sweep overlay: diagonal light gradient that slides across on switch (540-920ms)
- Auto-rotate: 4500ms interval, pauses on hover/focus, respects prefers-reduced-motion
- Empty gallery: script exits cleanly; single-image gallery initializes caption/tab state without starting auto-rotate
- Tabs: pill buttons 12px `--latin-ui`, active state uses brand-tint background
- Click navigation: left half = previous, right half = next
- Caption `.line`: italic serif, 14px olive. Poetic one-liners describing each screenshot
- Rapid switching keeps caption and tab state synchronized with the visible frame; test by clicking faster than the transition duration, not just once per panel

### Links

- One link behavior across the whole site: brand color, no underline, hover lightens. Do not add per-component underline or color exceptions; two special cases drift into five and the reader loses the "brand color means clickable" contract.
- That contract cuts both ways: brand color on non-links (FAQ questions, section titles) reads as clickable and misleads. Non-link headings stay near-black.

### Buttons

Two variants only:

| Variant | Background | Border | Text | Hover |
|---|---|---|---|---|
| `.btn-primary` | `--brand` | `--brand` | `--ivory` | `--brand-light`, translateY(-1px) |
| `.btn-ghost` | transparent | `--brand` | `--brand` | `--brand-tint` bg, translateY(-1px) |

Both: pill shape (999px radius), 15px `--latin-ui`, weight 500, 1.5px border, min-width 158px.

Mobile resting state: natural width, left-aligned to the hero text edge, height unchanged. Do not center them (reads as floating), do not stretch edge-to-edge with `flex: 1` (reads heavy), and never drop button height to relieve a "too full" feel; fullness is a width and spacing problem, not a height one. A left edge that does not line up with the text and chips reads as an accidental gap. Verify at 320px and 375px with no overflow.

### Pricing

Content rules in `references/writing.md` «Pricing rules»: benefits lead, the price states itself once, the CTA never carries the number.

- Benefits list `.price-benefits`: 4-6 items, serif 16px dark-warm, one line each at desktop width, plain borderless list (no bullets, no card frame)
- Amount: 30px serif, one factual line (`$N · One-time purchase` shape), `lining-nums tabular-nums`; never display-size the price. Emphasis comes from the centered block and whitespace, not font size
- The card stays quiet: no border box, no shadow; a gradient (if any) resolves to the page background, never to white
- CTA button label is the action (`Buy {{PRODUCT}}`), without the price
- Comparison: 18px, use `<s>` for competitor prices (stone color, 1px underline)
- Highlight: `.hl` class for brand-colored emphasis
- Terms: 13.5px olive, centered, max-width 640px, line-height 1.5

### Manifesto

- Brand philosophy paragraph: 20px, weight 400, line-height 1.55, letter-spacing 0.05em
- `<em>` renders in brand color with `font-style: normal` (brand emphasis, not typographic italic)

### Code Block

- `pre.code`: ivory background, 1px border, 6px radius, 18px 22px padding
- Font: `--mono` 13.5px, tabular-nums, line-height 1.55; reduce to 11.5px at the phone breakpoint (480px) so wide lines stay legible without horizontal scroll. `code { min-width: max-content }` lets long lines scroll instead of wrapping.
- Inline `code` is a distinct style, not the block palette: brand-tint background, brand text, 1px hairline, `0.9em`.

Screen code blocks may use a dark surface (`--shot-bg: #141318`, the same frame as the gallery) instead of ivory. Highlight at build time with zero runtime JS: a script bakes static `<span class>` markup (e.g. Pygments) and is idempotent, so re-running it after any doc edit refreshes the output; merge adjacent same-class spans so the markup stays small. Plain code stays the source of truth; the spans are generated, never hand-authored. Keep the token palette restrained on the dark surface:

| Token | Hex | Role |
|---|---|---|
| Comment | `#79756a` | faint, italic |
| Keyword | `#84aad6` | soft blue |
| String | `#8cbb91` | muted green |
| Number | `#cbab86` | sand |
| Function/Class | `#d6c78c` | sand-gold |
| Builtin/Constant | `#b59ccd` | muted violet |

Blocks without `class="language-*"` stay monochrome.

### Metrics

- Flex row with 32px gap, each metric is value (36px serif 500) + label (13px `--latin-ui` stone)
- `font-variant-numeric: tabular-nums` on values

### Cards (site-wide contract)

Like «Links» and «Buttons», cards are a vocabulary, not a per-section choice.

- One border treatment across every card on the site (same token, same width). Before styling a new card component, grep the page for existing card borders and reuse them; "this section's cards look different" is the most-reported drift.
- One hover treatment across every card: shadow or translate, pick one for the site. Two sections with different hover physics read as two different products.
- Re-verify foreground/background contrast in the hover state, not just the resting state. A hover that recolors the background can land text on a same-color background (this has shipped); any hover that changes either color must be checked with both.

### Demo Card Grid

- `auto-fill, minmax(240px, 1fr)` grid, 18px gap
- Cards: ivory bg, 1px border, 8px radius, whisper shadow on hover
- Image fills top, title 15px weight 500 + desc 12px olive below

### Content card lists (article / blog indexes)

For card grids whose content is written text (article listings, changelog indexes), length is controlled twice: at write time and at render time.

- Title: `-webkit-line-clamp: 2` with ellipsis as the hard stop, but write titles to fit one line; a two-line title is tolerated, not targeted. Titles carry no parenthetical asides (move them into the summary) and use a tighter line-height than body text (1.2-1.3).
- Summary: `-webkit-line-clamp: 4` with ellipsis. Never let overflow push card heights apart; the clamp keeps the grid rhythm even when content varies.
- Clamp plus ellipsis is the only truncation gesture: no fading text, no "read more" inline links inside the clamped block.

### Features

- Two-column grid: 200px name + 1fr description, 36px gap, separated by border-soft hairlines
- Feature name: 22px brand, weight 500
- Poetic subtitle: `<small>` below name, 13px olive, italic. One short line evoking the feature's character
- Description: 15px dark-warm, line-height 1.55
- Tables stay editorial: no framed box, no tinted header bar, no vertical rules, no empty right gap. Content-sized columns, hairline row rules, a muted `--latin-ui` uppercase header. On phone, `display: block; overflow-x: auto` rather than cramming columns. A framed, tinted table adds weight without adding information.

### Feature rows (a visual beside the copy)

The `.features` list above is the shipped default. A feature *row* (a visual on one side, the points on the other, repeated down the section) is a different component and no template ships it. It is also where a feature section most often goes wrong, because the instinct that produces it ("alternate the sides so it does not look like a product list") is the same thing that breaks it.

- **Compute the slack before building the row.** `slack = content width - (visual + gutter + the copy's natural width)`. The copy almost never fills a `1fr` track, so that number is positive on every row. Sizing the two tracks independently (a fixed visual track against a `1fr` copy track) does not remove it; it only decides which side it lands on.
- **Slack belongs on the outer trim, never in the gutter.** At the page edge it reads as a margin and disappears. Between the copy and the visual it reads as a hole, and it leaves the copy nearer the page edge than the thing it describes, which breaks the one association the row exists to make.
- **Mirror a text mass, never a short list.** A paragraph block has edges that read as a shape, so flipping which side it sits on costs nothing; that is why magazine spreads mirror. Four one-line points are not a mass. Their left edges are the only structure holding them together, and alternating rows move that edge every other row. Left edges are the strongest alignment cue on a page. Do not spend them on rhythm.
- **Three rescues that do not work.** Recorded so they are not retried. Pinning a fixed measure to the gutter: the slack turns into an unexplained indent on the mirrored rows. Right-aligning the mirrored copy so the bullet dots move to the right: flush against the visual, but the dots read as a mistake. Centering every seam on one shared axis: the visual leaves the page edge, the section loses its only anchor, and a third left edge appears.
- **Run every row the same way.** One visual edge flush with the section's left margin, one copy left edge, slack always at the outer trim. Nothing jumps, and the row that already read as fine is the row you keep.
- **Buy variety with weight, not with alternation.** A-B-A-B is still a pattern, and it produces a zigzagging list rather than editorial rhythm. Rhythm comes from unequal weight: a lead row with a larger visual or an opening sentence above its points, supporting rows with fewer and shorter points. If every row carries the same structure and near-identical copy length, no layout move will make the section feel unlike a list. The fix is in the content.
- **Measure the copy in every locale before capping the column.** The longest point's natural width swings hard by language (a CJK line can run 30% past its English source), and a system-font stack widens it again wherever the primary face is missing and a broader fallback takes over. No measure prevents wrapping everywhere, so do not chase one: pick the measure the layout needs, let long locales wrap, and follow «Cross-lang typography hardening».

### FAQ

- Wrap each dt/dd pair in `<div class="faq-pair">` for spacing (24px margin-bottom)
- `<dt>` question: 16px, weight 500, no top margin
- `<dd>` answer: 14px olive
- Code spans: mono 12px on brand-tint background, 3px radius
- Tail paragraph: `.faq-tail` after `</dl>`, 13px stone, links to help page. Closes the FAQ without another section

### Testimonial wall (only with real quotes)

No template ships this section; build it only from real, attributed quotes (name, handle or source). When the quotes flow in masonry columns with a collapsed default state:

- Clip the collapsed wall to the SHORTEST column so the fold is one flat line under the fade. Columns fill until each passes the collapse height and overshoot unevenly; clipped to the tallest, the expand control hangs off one column with dead space beside the others.
- One expand control ("Show all") below the fade; the expanded state removes the clip entirely. Expansion reveals everything at once (no pagination, no second click) and keeps the scroll position anchored at the control; jumping the viewport to the bottom of the expanded wall disorients the reader.
- Collapsed height is a conversion decision: roughly 9-12 quotes, and never so tall that the pricing section drops out of comfortable reach. Which quotes may appear collapsed is a content rule; see `references/writing.md` «Social proof rules».
- An external "more quotes" link (search results, social feed) renders as a card in the wall's own visual language, placed last, one line of copy; a styled button or foreign embed after the wall reads as an ad.

### Footer

- Two-column flex: brand mark (icon + name + tagline) left, colophon (links + ethos) right
- Mark icon: 56px rounded 8px
- Links: inline with middot (`&middot;`) separators between items, dark-warm color. Editorial pattern, not flex-gap
- Ethos: closing italic serif line, olive color, max-width 360px. The italic voice signals a personal sign-off
- Tech credit, once. If the product builds on an upstream project or framework, credit it exactly once as a quiet footer line, never as a repeated selling point and never in the hero tagline. Grep the whole site for the upstream name and collapse it to this single instance; rewrite the hero around the product's own positioning. Hard facts that are not the credit (license, version) belong here too.
- Collapses to single column below 880px

### Cross-lang typography hardening

- **Numeric alignment across CJK and Latin runs.** Use `font-variant-numeric: lining-nums tabular-nums` on every node that displays numbers (prices, metrics, version strings, tabular data). Lining keeps digit height uniform; tabular keeps digit width uniform. Without lining-nums, oldstyle fonts drop descenders on 3, 4, 5, 7, 9 and break vertical rhythm.
- **Latin fallback before CJK in the serif stack on Chinese pages.** Charter or Georgia first in `html[lang="zh-CN"] { --serif: ... }`, so mixed runs like "Mac/22/$19" share baseline with the Chinese body.
- **Avoid scaling the currency glyph with super.** Do not write `.price-currency { font-size: 0.5em; vertical-align: super }`. That trick makes `$` and the digit visually unequal. Prefer `font-size: 0.74em; line-height: 1; transform: translateY(0.015em);`.
- **Language menu items need vertical room for descenders.** When `<a>` inside `.lang-menu` has `line-height: 1`, the descender of 'g' / 'y' / 'p' is clipped. Use `min-height: 32px; padding: 6px 10px; line-height: 1.35;`. Add an invisible `::before` bridge between trigger and menu so the cursor can cross the gap without dismissing the menu.

> The main landing-page template ships a pricing section but no language switcher; the `{{HERO_LINKS}}` slot is where one would go. Kami's own `styles.css` ships a tested `.lang-switch` + `.lang-menu` implementation (hover bridge, descender padding, focus-within fallback); grep for `.lang-switch`. Copy it when you add multi-locale links to a landing page.

### Multilingual SEO scaffolding

- **hreflang link block in `<head>`.** One `<link rel="alternate" hreflang>` per shipped locale plus one `hreflang="x-default"`. Drop locales that have no actual page. Add `<link rel="alternate" type="text/plain" href="/llms.txt">` so AI assistants find the summary file.
- **og:locale + og:locale:alternate.** Self-reference the current page locale on `og:locale`, list the others on `og:locale:alternate`. Social previews on Facebook, LinkedIn, Telegram use this to pick the right thumbnail.
- **Canonical points at the per-locale URL.** Each locale should have a canonical that matches its own URL, not a single canonical pointing to `/`.

### Companion assets

The landing-page template alone is one HTML file. To deploy a production multilingual site you ship five companion files in the same folder; each is provided as `.example` and you remove the suffix before deploying:

- `landing-page-vercel.json.example`: path-based rewrites for `/zh`, `/tw`, `/ja`, `/ko`, host-canonical redirect, security headers, immutable cache for static assets.
- `landing-page-sitemap.xml.example`: one `<url>` per locale with `<xhtml:link>` cross-references; mirrors the hreflang block in `<head>`.
- `landing-page-robots.txt.example`: AI crawler allowlist (GPTBot, ClaudeBot, PerplexityBot, Applebot, OAI-SearchBot, Claude-SearchBot).
- `landing-page-llms.txt.example`: short brand summary; positioning, one-line competitor contrast, pricing, key links.
- `landing-page-llms-full.txt.example`: long-form companion AI assistants pull for accurate feature-level answers. Has Overview, Pricing, Features, Comparison, FAQ.

The optional Accept-Language redirect at the end of `landing-page-en.html` is commented out by default. Uncomment only after confirming `/zh/`, `/tw/`, `/ja/`, `/ko/` actually resolve on the host.

When a site uses generated locale pages, add a local drift check next to the generator. It should compare generated HTML to committed output, report missing placeholders by key, and fail before package or release work continues.

### Documentation site

When the product site grows docs, help, or guide pages (see «Product site system»), they need a layout the single landing page does not provide. All of this is screen-only.

- Two-column shell: a sticky sidebar nav plus a prose column. Sidebar around 178px, `position: sticky; top: 84px; max-height: calc(100svh - 108px); overflow: auto`. Constrain the prose column to a reading measure (about 720px) even though the page frame is wider; long doc lines hurt readability.
- Sidebar active state is a rail, not a fill. `border-left: 2px solid transparent` that fills brand on `[aria-current="page"]`, with brand text. No full-width dark underline or background block; that reads as a heavy dark bar against the warm paper.
- Multi-page topic structure: one file per topic, grouped under sidebar sections. Mark the current page with `aria-current="page"` so the rail and screen readers agree.
- On-this-page TOC: a thin in-flow list under a hairline top border, with a `--latin-ui` uppercase 11px "On this page" heading and depth-3 entries indented about 12px. Hide it entirely below the tablet breakpoint; it is an aid, not content.
- Prev/next pager: quiet borderless text links, not bordered cards. A 2-column grid with one thin top divider; each link is a `--latin-ui` uppercase "Previous"/"Next" eyebrow over a brand serif title, `border: 0; background: none`. The next link aligns right (resets left on phone). Press feedback via `:active { opacity: 0.6 }`. A bordered card here reads heavy on mobile.
- Mobile (tablet breakpoint): the sidebar un-sticks (`position: static`) and collapses to a horizontal scroll strip (`display: flex; overflow-x: auto; scrollbar-width: none`) with the active rail moved to `border-bottom`; the TOC is hidden. Reuse the landing page's existing breakpoints; do not invent a new ladder.
- Acceptance: at 1280px the prose column holds the ~720px measure; at 375px the sidebar reads as a horizontal strip with the active rail on `border-bottom` and zero page overflow; `aria-current` and the visual rail always agree.

### Dashboards (screen, no template)

Data dashboards built in the Kami idiom (analytics boards, sales views, status pages) inherit the full design system plus these information rules. No template ships; build from the landing-page shell.

- **One currency, one unit system.** Mixed currencies get converted to a single display currency before anything renders; a board that shows three currencies side by side cannot be scanned. Same for units (GB vs MB, days vs hours).
- **Zero rows disappear.** Regions, SKUs, or days with a zero value are hidden or aggregated into "other", not rendered as rows of zeros. Empty rows are noise wearing a grid.
- **One meaningful default time range.** Since-launch or last-30-days, chosen once; every selector option must answer a question the owner actually asks. Cut options rather than adding them.
- **Implementation detail stays out of the surface.** Sync mechanisms, API states, internal field names, and refresh plumbing belong in logs, not on the board. The reader is deciding, not debugging.
- **Every chart answers one question.** Where is growth, which region buys, what broke: if a chart only restates a number that is already on screen as a stat, delete the chart (screen twin of `references/anti-patterns.md` #18).
- Numbers on the board follow the reconciliation rule: totals must be checked against one authoritative source before shipping, with the basis noted.

## 12. Mermaid diagrams

Mermaid text is turned into Kami-styled diagrams via beautiful-mermaid plus
`scripts/mermaid_normalize.py`. The theme maps beautiful-mermaid's seven color
roles onto the canonical palette. `references/mermaid-theme.json` is the single
source, kept in sync with `tokens.json`; `references/mermaid.md` «The Kami theme»
prints the role-to-token mapping in readable form.

Same invariants as every other surface: parchment canvas, one chromatic accent
(ink-blue marks the focal element only), warm neutrals for everything else, serif
text with CJK fallback. The normalizer resolves beautiful-mermaid's `color-mix()`
derivations to static hex, so derived shades (e.g. `#dad9d3`) stay warm and never
introduce cool grays. PDF supports flowchart / state / sequence / class / ER;
`xychart-beta` is browser-only (it uses `<style>` class selectors WeasyPrint will
not apply). Full pipeline and rationale in `references/mermaid.md`.

### Responsive screenshot verification

Before declaring any screen change done, screenshot the real rendered surface; a type check or CSS-balance read is not enough. Several regressions (early wraps, orphaned separator dots, table overflow, missed pages) are invisible in source and only show in the render.

- Capture at phone (375px, plus 320px for CTAs) and desktop (1280px), in every shipped locale.
- Scan for line widows objectively: measure each text block's last-line width against its widest line and flag anything below about 13%. Eyeballing misses pages, and nested `<code>` hides widows from greps. Accept "0 widows" only after the check confirms it.
- Confirm CTAs reach their natural-width left-aligned resting state with no overflow, code is legible at the reduced mobile font, the gallery and any multi-column grids collapse to a single column, and total page overflow is zero.
- Scan each screenshot for sparse blocks: a low-information region taller than about a quarter viewport, an empty grid slot, or a single item rattling in a multi-column row. Fix by adding content or tightening the layout, not by leaving decorative whitespace.
- Check every «Single-line surfaces» entry at both widths; key-fact tokens (price, platform, CTA) must hold one line at 375px.
- Long pages do not fit one viewport; use a capture helper that can scroll to a specific element (first code block, pager) before shooting.
- Serve fresh bytes: browsers cache stylesheets and restore scroll positions, so a plain reload can screenshot the OLD css at the OLD scroll point and pass a broken change. Verify through a cache-busted URL (or a fresh-named temp copy of the page) and confirm the viewport actually shows the section under review before trusting the capture.

## KO locale tuning

Korean templates use Source Han Serif K (Adobe, also distributed by Google
as Noto Serif KR) as the primary serif. The font's hangul metrics are close
enough to TsangerJinKai (CN) that the CN per-component values render
naturally in Korean without per-template re-tuning. The `one-pager-ko`
pilot confirmed that the CN baseline values flow through cleanly: every
numeric value below matches the CN one-pager (and the rest of the CN
doc-style templates by extension), with one KO-specific micro-adjustment:
`.metric-label` font-size drops from 9pt to 7pt to accommodate the longer
mixed hangul + parenthesised-metadata labels typical of Korean editorial
content (see one-pager-ko `.metric-label` rule).

Canonical values (verified during the `one-pager-ko` pilot, 2026-05-28):

- Body `font-size`: 10pt (matches CN baseline)
- Body `line-height`: 1.45 (matches CN baseline)
- Body `letter-spacing`: 0.3pt (matches CN baseline)
- H1 `font-size`: 24pt (matches CN baseline)
- H1 `font-weight`: 500. CN templates use 500 (TsangerJinKai W05, a
  Medium-Bold) for every emphasis (body bold, headings, tags, metric
  values) and never reach for 700. Source Han Serif K exposes the full
  weight range (ExtraLight through Heavy), so KO bundles Regular (400) +
  Medium (500) to mirror CN's W04/W05 two-weight discipline. The result:
  KO emphasis reads at the same Medium tier as CN, rather than the heavier
  Bold the early Nanum-era forks settled on.
- H1 `letter-spacing`: matches CN per-template setting (typically 0 to −0.2pt
  on display H1s; copied from the CN sibling).
- H1 `line-height`: 1.15 (matches CN baseline)
- `.metric-label` `font-size`: 7pt (one-pager-ko only: KO labels read wider
  than CN/EN, so the baseline-flex metric strip needs a smaller label to
  avoid wrapping inside the card column).
- `font-synthesis: none;` MUST be applied to the body rule. WeasyPrint can
  synthesize fake bold when Bold weight resolution fails through fallbacks,
  and disabling synthesis keeps the editorial tone honest (real glyph
  shapes only).

Fallback chain (consistent across all KO templates):

```css
--serif: "Source Han Serif K", "Source Han Serif KR", "Noto Serif KR", "Apple SD Gothic Neo",
         "AppleMyungjo", Charter, Georgia, serif;
--sans:  var(--serif);
--mono:  "JetBrains Mono", "D2Coding", "SF Mono", "Fira Code",
         Consolas, Monaco, monospace;
--latin-ui: "Inter", -apple-system, "Segoe UI", Helvetica, Arial, sans-serif;
```

`"Source Han Serif K"` is the Adobe distribution name and the `@font-face`
declared name (so file/CDN loads resolve in a repo checkout or online).
`"Source Han Serif KR"` is the actual family name baked into the bundled OTFs
(nameID 1/16 = `Source Han Serif KR`, Korean `본명조 KR`); it MUST stay in the
chain so that on an offline Linux skill install -- where the relative
`@font-face` file is stripped and jsDelivr is unreachable -- fontconfig can
still resolve the `ensure-fonts.sh`-downloaded OTF by name (the bare
`Source Han Serif K` matches nothing). `"Noto Serif KR"` is the Google Fonts
name for the same Adobe source, covering boxes that installed it via
`fonts-noto-cjk`. Listing all three keeps the chain agnostic to which
installer the user used.

Subsequent KO templates (letter-ko, long-doc-ko, etc.) should adopt the
font variables and `font-synthesis` rule verbatim and leave all numeric
values at their CN sibling's baseline.
