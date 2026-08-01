# Deck pre-flight (slides only)

Loaded from `SKILL.md` Step 2.6 when the document type is slides. Every other doc type skips this file entirely.

Read `references/design.md` Section 8 «Deck Recipe» alongside this file before drafting: it owns the slide architecture, typography, layout patterns, table styles, and SVG constraints.

## Path selection

Default to the WeasyPrint HTML path. Switch to pptx only if the user explicitly requires an editable PPTX file. Switch to Marp only when the user explicitly asks for Marp / markdown slides.

| Path | Template | When |
|---|---|---|
| WeasyPrint HTML → PDF (default) | `slides-weasy.html` / `slides-weasy-en.html` / `slides-weasy-ko.html` | All cases unless PPTX or Marp is required |
| python-pptx → PPTX (fallback) | `slides.py` / `slides-en.py` | User explicitly requires editable PPTX |
| Marp Markdown (variant) | `assets/templates/marp/slides-marp.md` (+ `slides-marp.css`) / `slides-marp-en.md` (+ `slides-marp-en.css`) | User explicitly asks for Marp, "markdown slides", or a `.md` deck. Shipped `.md` is a working demo of Kami Marp itself; copy it, swap content, keep the structure. Renders via local `marp` CLI; not bundled. |

## Page size

Default is `280mm 158mm`. Ask only if the user has mentioned length or density constraints.

| Size | When |
|---|---|
| `280mm 158mm` | Default; fits most decks |
| `297mm 167mm` | User wants a bit more room |
| `338mm 190mm` | Heavy content slide or many data points per page |

## Content pre-flight

Before drafting any slide, confirm these points with the user. Ask all at once, skip any already answered:

| # | Question |
|---|---|
| 1 | **Audience + venue** - who is in the room, and is it live keynote, investor 1:1, or async share link? |
| 2 | **Length target** - presentation time or slide count? (15 min: ~10 slides / 30 min: ~20 slides / 45 min: ~25-30 slides) |
| 3 | **Source material** - what content is already ready: outline, doc, notes, data? |
| 4 | **Images** - are screenshots, charts, logos, or product images available; which slides need real evidence slots; and is a separate visual brief needed? |
| 5 | **Hard constraints** - brand colors, required logo, PPTX required, any slides that must exist? |
| 6 | **Format confirmation** - slides deck, or a one-pager that looks like a deck? |

## Content rules for slides

- Ghost deck test: read only the slide titles in order. They must tell the argument; if not, fix titles or structure before styling
- One evidence shape per slide: chart, table, screenshot, code, quote, or conclusion. Split mixed evidence instead of crowding one slide
- Audience copy stays clean: titles, body, and captions never contain image prompts, crop instructions, or generation notes
- No section divider slides: use `.eyebrow` for section numbering, not a dedicated blue-background page
- No CJK parentheses: replace `（...）` with `·` or `,`
- Each bullet fits one line: trim until it does
- 2×2 layouts: use `table.t2x2`, not CSS Grid
- Pinned conclusions: use `.co` at `position: absolute; bottom: 12mm`

These rules apply identically to Marp decks. Marp-specific syntax: see `references/design.md` §8 «Marp variant».
