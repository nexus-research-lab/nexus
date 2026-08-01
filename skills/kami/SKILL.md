---
name: kami
title: Kami 文档设计
version: 1.11.0
category_key: content-docs
category_name: 内容与文档
tags:
  - document-design
  - pdf
  - slides
  - landing-page
description: >-
  使用统一的暖纸张、墨蓝强调色和衬线层级，排版专业简历、一页纸、白皮书、正式信件、
  作品集、研报、更新日志、幻灯片与产品落地页。用户要求做 PDF、排版、一页纸、白皮书、
  作品集、简历、PPT、slides、Marp、落地页、官网，或希望把内容整理成可直接交付的编辑式
  成品时使用。中文、英文、韩文有专用模板，日文走 CJK 最佳努力路径；不用于动态 Web App、
  深色赛博风或高饱和多色视觉。
recommendation: 适合把简历、报告、幻灯片与落地页排成统一、可验证的编辑式成品。
allowed-tools:
  - Read
  - Write
  - Bash
  - WebFetch
---

# kami · 紙

**紙 · かみ** - the paper your deliverables land on.

Good content deserves good paper. One design language across documents and landing pages: warm parchment canvas, ink-blue accent, serif-led hierarchy, tight editorial rhythm.

Part of `Kaku · Waza · Kami` - Kaku writes code, Waza drills habits, **Kami delivers documents**.

> **来源**：本内置 Skill 完整引入 Tw93 的 [`Kami`](https://github.com/tw93/Kami) 轻量
> Skill 包，固定来源提交为
> [`a211e7bbf9470493debfbbf0fe4645c05eaa546f`](https://github.com/tw93/Kami/commit/a211e7bbf9470493debfbbf0fe4645c05eaa546f)，
> 采用 MIT 许可证。完整来源与本地改动见 [SOURCE.md](SOURCE.md)，许可证见
> [LICENSE](LICENSE)，字体与可选依赖说明见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。

**Nexus path rule.** Resolve every `assets/`, `references/`, and `scripts/` path from
`${CLAUDE_SKILL_DIR}`. Copy the selected template into the user's working directory before editing;
never write generated documents or build artifacts back into this built-in Skill directory.

**Pinned built-in release.** Do not run `scripts/check-update.sh` or refresh plugin packages. Nexus
updates this Skill only through a reviewed source bump, so the bundled version stays deterministic.

**Dependencies.** Do not install packages automatically. HTML templates and static checks work with
Python 3.10+. PDF rendering and perceptual checks need the packages listed in `requirements.txt`;
syntax highlighting and editable PPTX are listed in `requirements-optional.txt`. If a requested output
needs a missing dependency, show the relevant command and let the user install it:

```bash
python3 -m pip install -r "${CLAUDE_SKILL_DIR}/requirements.txt"
python3 -m pip install -r "${CLAUDE_SKILL_DIR}/requirements-optional.txt"  # optional
```

## Step 0 · Load brand profile (if exists)

Check `~/.config/kami/brand.md` (preferred) or `~/.kami/brand.md` (legacy fallback). If found, read `references/brand-profile.md` for the full four-layer application spec (placeholder substitution, session defaults, visual customization, habit notes) and its six guardrails. If no profile exists, continue without interruption.

Key rule: explicit prompt > editorial judgment > habit notes > frontmatter defaults > built-in defaults. Profile fills gaps silently; it never overrides the current conversation.

## Step 0.5 · User project style scan (opt-in)

Run this only when the user explicitly references a sibling project as a visual reference: "like my <project> site", "match the style of <repo>", "use the look from <directory>". Skip silently when no such reference exists.

When triggered, before generating: locate the referenced project's declared design tokens (CSS variables, tailwind/theme config, token files) and extract dominant colors, font stack, spacing scale, and border-radius scale; prefer declared tokens over inline literals. Merge into the in-session brand profile as Layer C (visual customization), not Layer B (session defaults); never override an explicit `--brand` flag or values the user typed this turn. Report back in one line before continuing: "scanned <project>, extracted N colors / M fonts; using as visual reference."

Skip and fall back to the brand profile defaults if the referenced path does not exist, no CSS-like files are found, or the extraction would conflict with the user's explicit values in the current message.

---

## Step 1 · Decide the language

**Match the user's language.** Chinese -> `*.html` / `slides-weasy.html`. English -> `*-en.html` / `slides-weasy-en.html`. Japanese -> CJK path (`.html` / `slides-weasy.html`) as best-effort, JP Mincho first, visual QA before shipping. Korean -> dedicated `*-ko.html` / `slides-weasy-ko.html` family as best-effort, visual QA before shipping. Reference docs are shared English specs.

| User language | HTML templates | Slides (PDF default) | Slides (PPTX fallback) |
|---|---|---|---|
| Chinese (primary) | `*.html` | `slides-weasy.html` | `slides.py` |
| English | `*-en.html` | `slides-weasy-en.html` | `slides-en.py` |
| Japanese (best-effort) | `*.html` | `slides-weasy.html` | `slides.py` |
| Korean (best-effort) | `*-ko.html` | `slides-weasy-ko.html` | n/a (use `slides-en.py` only if PPTX is required) |
| Other languages (best-effort) | choose CJK or EN path by script coverage, then verify manually | choose `slides-weasy.html` or `slides-weasy-en.html`, then verify manually | use `slides.py` / `slides-en.py` only if PPTX is required |

> Default to the WeasyPrint HTML path; fall back to PPTX (`slides*.py`) only when the user explicitly needs an editable deck.

Design, writing, production, and diagram guidance live in `CHEATSHEET.md` and `references/*.md`. Step 3's tier table decides how much of it to open; read the lowest tier that covers the task, not all of it.

Code blocks with `class="language-*"` are highlighted only when optional `Pygments` is installed in the build environment. Without it, PDFs still render and code blocks stay monochrome.

## Step 1.5 · Intent extraction (silent)

Before picking a template, silently confirm purpose, audience, hard constraints, and what outcome counts as success. Skip any dimension the conversation or the document type already answers (a resume's purpose is always "get an interview").

Question budget, global for this skill: infer first, from the decision tree and context. Ask at most once, in one compact question (max 2 sub-questions), and only when 2+ intent dimensions are genuinely unresolvable or two templates genuinely both fit. Never present the dimensions as a checklist.

## Execution contract

Before creating or modifying an output, lock the contract: language, template, output format, page or length target, visual acceptance check, and verification command. Infer from the user's request when clear; ask only when missing fields materially change the deliverable.

Use the nearest existing template and verification path. Do not add a new template, shared CSS layer, dependency, script flag, or optional mode unless the current request cannot be satisfied without it.

Nexus ships this directory directly; do not build or refresh upstream `dist/kami.zip` or plugin
marketplace metadata as part of a document task.

---

## Step 2 · Pick the document type

| User says | Document | CN template | EN template | KO template |
|---|---|---|---|---|
| "one-pager / 方案 / 执行摘要 / exec summary" | One-Pager | `one-pager.html` | `one-pager-en.html` | `one-pager-ko.html` |
| "white paper / 白皮书 / 长文 / 年度总结 / technical report" | Long Doc | `long-doc.html` | `long-doc-en.html` | `long-doc-ko.html` |
| "formal letter / 信件 / 辞职信 / 推荐信 / memo" | Letter | `letter.html` | `letter-en.html` | `letter-ko.html` |
| "portfolio / 作品集 / case studies" | Portfolio | `portfolio.html` | `portfolio-en.html` | `portfolio-ko.html` |
| "resume / CV / 简历 / 履歴書" | Resume | `resume.html` | `resume-en.html` | `resume-ko.html` |
| "slides / PPT / deck / 演示" | Slides | `slides-weasy.html` | `slides-weasy-en.html` | `slides-weasy-ko.html` |
| "个股研报 / equity report / 估值分析 / investment memo / 股票分析" | Equity Report | `equity-report.html` | `equity-report-en.html` | `equity-report-ko.html` |
| "更新日志 / changelog / release notes / 版本记录" | Changelog | `changelog.html` | `changelog-en.html` | `changelog-ko.html` |
| "landing page / 落地页 / 官网 / product page / 产品页" | Landing Page | `landing-page.html` | `landing-page-en.html` | `landing-page-ko.html` |

> **Changelog vs. release notes**: The changelog template above is for styled document output. GitHub release notes are a separate deliverable; use `/write` with Release Note Template Mode.

> **Landing Page**: Screen-first interactive template. No PDF output. Includes gallery carousel with auto-rotate, hero entrance animation, responsive breakpoints (880px / 480px), and prefers-reduced-motion support. Deploy as static HTML to Vercel / Netlify / any host. The agent fills {{PLACEHOLDER}} values and HTML comment blocks, then saves as a ready-to-serve `.html` file.

> **Landing Page companion files**: For a production multilingual deploy, copy the five `landing-page-*.example` files alongside the main HTML and drop the `.example` suffix. `{{SITE_ORIGIN}}` is the scheme + host of your `{{CANONICAL_URL}}` (e.g. `https://example.com`). What each file covers, and the opt-in Accept-Language redirect: `references/design.md` Section 11 «Companion assets».

> **Production product site mode**: docs, help, releases, changelog, roadmap, legal pages, or more than two locales means a site system, not a page. Lock the five items below from the source material before filling any template, asking only where a missing item would change the deliverable. Then follow `references/design.md` Section 11 «Product site system», and «Documentation site» in the same section once the page grows a docs or help shell.

| # | Lock |
|---|---|
| 1 | **Product category** - first-viewport category: app, CLI, terminal, utility, skill, template system, or another user-provided label. |
| 2 | **Real assets** - available product screenshots, logo, icon, or UI captures, mapped to hero/gallery/feature/social slots. Missing assets must stay marked, not replaced with stock imagery. |
| 3 | **Site shape** - single page, or home plus docs/help/releases/changelog/roadmap/legal pages? |
| 4 | **Locales** - exact locale list, canonical paths, and whether a generator/check mode is needed. |
| 5 | **Truth surfaces** - install path, price, version, support route, FAQ, `llms.txt`, and `llms-full.txt` that must stay synchronized. |

Keep project-specific release artifacts, payment providers, appcast rules, and private local paths out of Kami.

### Decision tree (use before asking)

Ask only when two cells genuinely both fit.

| Signal | Document |
|---|---|
| Length target unknown | Ask "how many pages" before classifying |
| ≤ 1 page + investor / recruiter / exec summary audience | one-pager |
| ≤ 1 page + formal correspondence (sales, hiring, resignation, memo) | letter |
| 1.5-2 pages + career narrative + project bullets | resume |
| 3-6 pages + project showcase + visual heavy | portfolio |
| 6-15 pages + sustained argument + low visual density | long-doc |
| Presentation flow + speaker support + per-slide assertion | slides |
| Financial / metrics dashboard + thesis + price or risk view | equity-report |
| Version-by-version log + release facts | changelog |
| Product showcase + pricing + screenshots + FAQ for browser | landing-page |

Ambiguity examples that justify a one-liner:
- "1.5 page career story with heavy visuals" -> ask "resume or portfolio?"
- "2 page exec summary with metric tiles" -> ask "one-pager or equity-report?"
- "5 page argument with several charts" -> ask "long-doc or portfolio?"

### Diagrams (primitives, not a separate template type)

When the user asks for **a diagram inside** a long-doc / portfolio / slide (not a standalone document), route to `assets/diagrams/` rather than a template. Eighteen types ship there; `references/diagrams.md` section 1 «Selection» maps each to what it shows, and section 7 lists the AI-slop patterns to avoid. Read it before drawing. Extract the `<svg>` block from the chosen file and drop it into a `<figure>` inside the document.

Three routes inside that file, by trigger:

| Trigger | Section |
|---|---|
| Full-system panorama, control plane, roadmap, or owner map in one artifact | «Architecture boards». Do not inflate the single architecture figure past its node budget. |
| A diagram that lives in the user's repo (README figure, "给项目画张架构图", updating an existing one) | «Maintained diagram assets». Evidence pass first, keep the `index.html` + PNG + `prompt.md` trio consistent, never redraw from memory or hand-edit the PNG. |
| A standalone raster illustration or a redraw of a photo or screenshot | «Illustration briefs», plus the Illustrations rules below. |

**Auto-select charts from data.** When content carries numbers, pick the chart type yourself and embed it without asking. Two house calls that differ from the common default: a ~100% share renders as a donut only up to 6 items, and 7 or more becomes a horizontal bar; a single time series whose absolute count changes dominate (not rate) renders as bars, not a line. When several types fit, prefer the one that shows variance most clearly. Always embed inside a `<figure>` whose caption states the insight, not the data range.

### Illustrations (host image model, not inline SVG)

Inline diagrams above are vector SVGs you assemble by hand. For a standalone raster illustration, or a redraw of a figure, photo, or screenshot in the Kami look, delegate the drawing to the host's own image generation. Never call an external image API or require a key; rendering is the host's job.

- If the running host can generate images (for example ChatGPT), apply the brief below and render the image directly.
- If it cannot (Claude, Codex, most coding agents), output the brief as text so the user can paste it into any image model.

Brief: warm parchment (`#f5f4ed`) background, never pure white; one accent only, ink blue (`#1B365D`); all else warm gray with a yellow-brown undertone, no other colors; thin single-line geometric strokes and simple flat icons; no gradients, drop shadows, or 3D; serif labels; generous whitespace, composed like a figure in a well-typeset report. Full brief skeleton, icon rules, and QC checklist: `references/diagrams.md` «Illustration briefs».

Switch to this path (instead of enlarging a hand-assembled SVG) when a figure needs more detail than SVG assembly holds at the target display width, typically web-article figures at teaching depth (see `references/diagrams.md` density tiers). When one deliverable needs several generated images, drive them through a single handoff file: one line per image (slot, aspect ratio, shared style anchor, prompt, status), generate in batches of at most 5, update the status column after each batch, and check existing generated output before regenerating. The style anchor is shared by the whole batch; per-image style drift is the failure mode.

## Step 2.1 · Source and material pass

Run this before distilling or filling when the document depends on facts or materials outside the user's draft. Skip it for personal drafts where the user already supplied everything.

**Source check** fires when the document names a company, product, person, release date, version, funding round, metric, market fact, or technical spec. Work from primary sources, note the source and date for facts that drive the document, and ask the user when sources conflict rather than choosing silently. `references/writing.md` «5. Sources before phrasing» owns the detail, including which current-sounding claims are banned until checked ("latest", "recent", "new", version numbers, launch dates, financial figures).

**Material check** fires when the subject is a company, product, project, venue, or personal brand. Confirm what makes it recognizable before layout:

| Need | Required when | Accept |
|---|---|---|
| Logo | Any branded document | User file or official SVG/PNG |
| Product image | Physical product / venue / object | Official image, user image, or marked gap |
| UI screenshot | App / SaaS / website / tool | Current screenshot, official product image, or user capture |
| Brand colors | Branded one-pager / portfolio / deck | Official value, extracted asset value, or keep kami ink-blue |
| Fonts | Only if brand typography matters | Official font, close system fallback, or kami default |

A missing item gets a compact gap table and one question, never a generic stand-in, an approximated logo, or an invented value. `references/writing.md` «6. Materials serve recognition» carries the writing-side rules.

Logo fallback: when the request names no logo but the brand profile has a `logo` path, fill the commented `.brand-logo` slot in `one-pager` / `portfolio` / `slides-weasy` per `references/brand-profile.md` Layer C. Expand `~` to an absolute path; if the file is missing or the template has no slot, leave it commented and render without a logo, never a broken image. An explicit logo in the current request always wins.

### Materials status block

After the material check, output a structured status block before continuing. This is a one-shot transparency display, not a question:

```
Materials status:
- Logo: OK assets/client-logo.svg
- Brand colors: OK #1B365D mapped to --brand
- Product screenshot: MISSING (proceeding with kami default placeholder)
- UI screenshot: not required for this doc type
```

Use `OK`, `MISSING`, or `not required`. If a required item is missing and no user input arrived, ask once with the gap table; otherwise continue silently.

## Step 2.5 · Distill raw content (if applicable)

**Auto-detect whether to distill.** Do not ask the user; judge from the input. Skip distill only when the content already maps one-to-one onto the template's sections with quantified metrics in place, or the user said "use as-is" / "直接用这个". When in doubt, run distill.

Distill acceptance: every factual claim, number, date, name, source, and material reference from the raw input must land either in a template section (per-doc structure in `references/writing.md`) or in a gap table of what the template needs but the content lacks. Share the gap table once; never guess to fill a gap.

Example gap-check:

| Template needs | Found | Missing |
|---|---|---|
| 4 metric cards | "8 years", "50-person team" | 2 more quantifiable results |
| 3-5 core projects | 2 mentioned | at least 1 more with outcome |
| Materials | logo file provided | product screenshot source |

Then proceed to Step 2.6 (slides) or the layout note (all other doc types) with structured, distilled content.

### Persist the distilled content as a content IR (new documents)

When building a new document (not a text tweak on an existing one), write the distilled result to a `content.json` next to your working HTML before filling:

```json
{"type": "resume", "lang": "cn", "content": { ... }}
```

`type` is one of the schema names in `references/schemas/` (one-pager, letter, resume, long-doc, portfolio, slides, equity-report, changelog, landing-page). Read the matching schema before writing: its `$comment` notes carry the per-field quality bar. Then validate before any layout work:

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-content content.json
```

Fix schema errors by fixing the content (or asking the user for the missing fact), not by loosening structure. After filling the template, re-run with the filled HTML to confirm no fact was dropped on the way in:

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-content content.json filled.html
```

Values longer than 80 characters are treated as prose you may rephrase; short atomic values (names, metrics, dates, bullets) must survive verbatim.

## Step 2.6 · Deck pre-flight (slides only)

Slides only. Every other doc type skips to Step 2.7.

Load `references/deck-preflight.md` and work it before drafting: path selection (WeasyPrint HTML by default), page size, the six pre-flight questions to ask in one batch, and the slide content rules.

## Step 2.7 · Layout note (transparent, non-blocking)

Before loading specs and filling the template, write a short editor-style note stating the layout intent: template choice, length target, narrative arc, embedded diagrams, material status, and output formats. Match the document's language. Keep it under 80 words, written as prose, not a status panel. Continue immediately after; do not wait.

Example (CN):

> 排版意图：Equity Report 中文版，2 页 A4。先立论与目标价，进入估值 (DCF 与可比公司)，落于催化剂与风险。中段嵌一张营收趋势折线和 FY26 收入桥瀑布。Logo 已就位，产品图暂缺，header 改走纯文字。输出 HTML 与 PDF。

Example (EN):

> Layout intent: Equity Report (EN), two pages A4. Open with thesis and price target, run through valuation (DCF and comparables), close on catalysts and risks. A revenue line chart and an FY26 waterfall sit mid-doc. Logo is in hand; product image is absent, so the header stays text-only. Output: HTML and PDF.

If the user pushes back, adjust; otherwise proceed to Step 3.

---

## Step 3 · Load the right amount of spec

Pick the tier that matches the task. Default to the lowest tier that covers the work.

| Tier | When | Read |
|---|---|---|
| **Content-only** | Updating text, swapping bullets, translating an existing doc. CSS stays untouched. | `CHEATSHEET.md` only |
| **Layout tweak** | Adjusting spacing, moving sections, changing font size within spec. CSS touched. | `CHEATSHEET.md` + template (tokens already inline) |
| **New document** | Building from scratch or from raw content. | Full design spec + writing spec + template + `references/schemas/<type>.json` |
| **Resume content** | Resume-specific bullet structure, project framing, scope-result-outcome rules. | `resume-writing.md` + template |
| **Sources / materials** | Company, product, market, launch, funding, specs, or branded subject. | `writing.md` source rules + user/source material |
| **Deck (>20 slides)** | Long presentation needing Part Divider, Code Cards, section headers. | Full design spec + Deck Recipe (design.md section 8) |
| **Troubleshoot** | Rendering bug, font issue, page overflow. | `production.md` (+ design spec if CSS is the cause) |
| **Anti-patterns** | Reviewing AI-generated drafts before shipping. | `anti-patterns.md` (nine-category checklist) |
| **Diagram** | Embedding SVG in a doc, or maintaining a repo-owned diagram (trio: HTML + PNG + prompt.md). | `diagrams.md` (has its own token map); add `references/mermaid.md` when the source is Mermaid text, and `references/production.md` Part 4 when a figure renders wrong in the PDF |


## Step 4 · Fill content into the template

- Copy the template into your working directory; don't write HTML from scratch
- **CSS stays untouched during content fill.** Layout adjustments go through the Layout-tweak tier (Step 3) and stay within spec; any real style change syncs `references/design.md` and the sibling templates, never a single file
- Content follows `writing.md`: data over adjectives, distinctive phrasing over industry clichés
- Avoid patterns listed in `references/anti-patterns.md`: emptiness, fabrication, mimicry, excess, source gaps, tone contamination, landing page, image slots, slides
- **Before filling, read the quality bar for your document type** in `writing.md` section "Quality bars by document type". Structure is necessary but not sufficient: a resume bullet needs Action + Scope + Result + Business Outcome; an equity report needs variant perception + quantified catalysts; slides need assertion-evidence titles. Meeting the quality bar is as important as filling every placeholder.

### Do not generate

These are the most common AI document failures. Cross-reference `references/anti-patterns.md` for the full list; `--check-placeholders` catches leftover template tokens mechanically.

- Do not invent metrics, financial data, or statistics; mark gaps with `[DATA NEEDED: description]`
- Do not use stock-image descriptions as image placeholders ("A diverse team collaborating in a modern office")
- Do not pad content to fill template slots (a resume with 3 real projects does not need 5 fabricated ones)

### Fill PDF metadata (WeasyPrint reads these into the PDF)

Every template has meta placeholders in `<head>`. Fill all four before building:

| Placeholder (CN) | Placeholder (EN) | Rule |
|---|---|---|
| `{{作者}}` | `{{AUTHOR}}` | Resume/letter/portfolio: use the person's name from the doc. All others: leave as-is (build script infers from git config or env) |
| `{{摘要}}` | `{{DESCRIPTION}}` | Extract one sentence (≤150 chars) from the first 2 paragraphs |
| `{{关键词}}` | `{{KEYWORDS}}` | 3-5 keywords from the title + section headings, comma-separated |
| `{{文档标题}}` / `{{信件主题}}` etc. | `{{DOC_TITLE}}` / `{{LETTER_SUBJECT}}` etc. | Infer from the H1 or `.header .title` text |

`<meta name="generator" content="Kami">` is already fixed in the template; do not change it.

**Author inference**: `build.py` automatically sets PDF `/Author` metadata from:
1. `git config user.name` (primary)
2. `KAMI_AUTHOR` environment variable (fallback)
3. `"Kami"` (final fallback)

For personal documents (resume/letter/portfolio), the HTML `<meta name="author">` should match the person's name in the content. For non-personal documents (one-pager/long-doc), leave the placeholder as-is and let the build script infer it.

## Step 4.1 · Per-page density target (multi-page templates only)

适用：slides-weasy / long-doc / portfolio / equity-report / changelog。不适用 resume / one-pager / letter（这些有独立的长度合约）。

正文页填充率目标 60-80%。封面 / 目录 / 末尾署名页豁免。这条规则解决的是 AI 生成多页文档时最常见的 draft 缺陷：把内容拆得太散，结果几页都填不满。

### Items-per-page contract

| Template | Typical body page | Hard floor (merge if below) |
|---|---|---|
| slides-weasy | 1 assertion title + 3-5 supporting items, or 1 chart + 2-3 callouts | <3 items and no chart → merge into adjacent slide |
| long-doc | 1 chapter heading + 2-4 paragraphs + at most 1 figure | Chapter renders to <40% page → merge into neighbor chapter |
| portfolio | 1 project header + 1 hero image + 3-5 outcome bullets | No image and <3 outcomes → merge with adjacent project |
| equity-report | 1 section + 1 table/chart + supporting prose | Only a 2-row table on the page → combine sections |
| changelog | 1 version block + 4-8 entries | Version has <4 entries → place on the same page as the prior version |

### Sparse-page merge rule

Before finalizing, scan the draft. Any body page that would render under 50% full → apply one of, in order:

1. Merge upward into the previous section.
2. Merge downward into the next section.
3. Promote a list to a small diagram or table that earns the space.
4. Pin a `.co` callout to bottom (slides-weasy only). Whitespace above a pinned callout is intentional, not sparse.

Forbidden ways to "fill" a sparse page: padding with filler prose, repeating the heading as a sentence, inventing statistics, restating the prior page in different words. If the merge options don't apply, the page itself shouldn't exist.

### Last-page exemption

The last body page is allowed to run 40-60% fill. Forcing balance on the last page usually means padding. The colophon / closing slide may have any fill level.

### Verify after build

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-density   # flags >25% (WARN) / >50% (SPARSE) trailing whitespace
```

If a body page (not cover, not last page) gets a SPARSE warning, treat it as a draft defect and re-author with the merge rule.

## Step 4.2 · Resume recruiter pass (resume only)

Mechanical checks (`--check-placeholders`, `--check-resume-balance`, `--check-density`) validate structure and layout, not prose. A resume can pass all of them and still read broken. After filling and before building, reread every project card the way a recruiter would, against the row definitions in `references/resume-writing.md` ("What goes in each row"): Role carries your position in the project, not background alone; Actions are verb-led, one concrete approach per sentence; Impact reads as an outcome, not a restatement of the process. One cross-row check on top: no row repeats another row's information.

Fix a failing row by rewriting from the source material. If the source cannot support a row (for example, no outcome fact exists), ask the user for the missing fact. Do not pad, and do not fall back to generic claims ("保障稳定运行", "improved efficiency").

This pass is internal: run it silently; surface it only when a row cannot be fixed without new information from the user.

## Step 4.5 · Auto-select output format

Do not ask the user which format to export. Decide from context:

| Signal | Output | Why |
|---|---|---|
| Any document request | HTML + PDF | PDF is the default deliverable, HTML is the source |
| Slides / PPT / deck | HTML + PDF + PPTX | Presentations need a projectable format |
| "分享" / "发朋友圈" / "share" / "post" / "preview" | + PNG | Social platforms and messaging need images |
| "嵌入" / "插图" / "embed in another doc" | PNG only | Used as material inside other documents |
| User explicitly says a format | Follow the user | Explicit request overrides auto-selection |

PDF always ships for document templates. Landing pages ship as a ready-to-serve static HTML file. PPTX follows slides. PNG follows sharing context.

## Step 5 · Build & verify

```bash
python3 "${CLAUDE_SKILL_DIR}/scripts/render_document.py" path/to/filled.html --output path/to/filled.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-placeholders path/to/filled.html
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-markdown path/to/filled.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-content content.json path/to/filled.html
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-visual path/to/filled.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-fonts path/to/filled.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-style path/to/filled.html
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-resume-balance path/to/resume.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-density path/to/filled.pdf
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-rhythm slides slides-en
python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check
```

The Nexus wrapper renders the user's copied HTML directly to the requested output path. Upstream
`build.py --verify` and named template targets write into `assets/examples/`; they are maintainer checks
and must not be used against the shared built-in Skill during a normal document task.

> **Screen verify**: `--check-density` is a print gate. For ANY browser-delivered surface (landing page, docs page, dashboard, testimonial wall, article index), screenshotting the rendered page at 375px and 1280px in every locale is a hard step before declaring done, not an on-request extra: scan for line widows, sparse blocks, and single-line-surface wraps, and report the result. Do not wait for the user to ask "does it work on mobile". See `references/design.md` Section 12 «Responsive screenshot verification».

> **Perceptual verify (PDF deliverables)**: geometry checks cannot see a fallback glyph or an arrow crossing a label. Before shipping a filled PDF, run `python3 "${CLAUDE_SKILL_DIR}/scripts/build.py" --check-visual path/to/filled.pdf`, then view every exported page image against the printed checklist. One hit means a whole-document sweep for that class of issue. If your host cannot view images, send the image paths and checklist to the user instead of skipping the pass. `--check-visual` runs the font gate for you and prints its verdict above the checklist.

> **Font verify (CJK deliverables)**: a missing CJK serif produces no fallback boxes. It silently substitutes a sans that still reads, so the page passes an eyeball pass while carrying typography the parchment metrics were never tuned for, and the result reads heavy and flat without anything looking obviously broken. `--check-fonts` settles it from the rendered PDF's own span table: it names the family that drew the body ideographs and fails on a sans substitution or on text split across two families. Never report a CJK document as visually verified without it, and never assume a sandbox has the fonts: the commercial TsangerJinKai02 files never ship inside the skill package.

Source templates intentionally keep `{{...}}` fields. Run placeholder checks on completed documents, not on the template library.

For Markdown-sourced long documents, also run `--check-markdown` on the rendered PDF. It catches visible raw `---`, `**bold**`, and inline-code backticks that should have been converted or removed before delivery.

Visual anomalies (tag double rectangle, font fallback, page break issues) -> `production.md` Part 4.

### Definition of done

A task is done when the user receives, in the closing message:

1. The path of every deliverable, in every promised format (Step 4.5).
2. Which checks ran and their results, including the page-count contract.
3. Every remaining `[DATA NEEDED]` gap, listed explicitly. Never declare done with an unreported gap.
4. The visual verdict, stated honestly by surface: for PDFs the `--check-visual` pass status (which includes the font gate); for screen surfaces the 375px/1280px screenshot result; when rendering could not be inspected, say "build verified, visuals unconfirmed", not "done".

## Fonts

When `--check-fonts` reports a sans substitution or a split family, read
`${CLAUDE_SKILL_DIR}/references/production.md` Part 1 «Fonts». Do not run
`${CLAUDE_SKILL_DIR}/scripts/ensure-fonts.sh` silently: TsangerJinKai02 is personal-use-only and
requires a separate commercial license. Tell the user, obtain confirmation, then either run the script
for an authorized personal use or use an installed open/system serif fallback. Waiting for visible
fallback glyphs is not a strategy; the common failure has none.

Two facts worth carrying here: the commercial TsangerJinKai02 files stay in the repo for local preview and CDN fallback but never go inside a Claude Desktop skill ZIP, and `Source Han Serif KR` is the real family name inside the OTFs, so it must stay in the KO chain for offline fontconfig to resolve it.

## Feedback protocol

When the user gives **vague visual feedback** ("looks off", "太挤了", "not elegant"), do not guess. Ask back naming the element and its current value, offering 2 in-spec alternatives.

Template response: "X is currently set to Y. Would you like (a) [specific alternative within spec] or (b) [another option]?"

Never say "I'll adjust the spacing" without naming the exact property and its new value.

**Escalate after two rounds.** If the same element is still not approved after two adjustment rounds, stop nudging values: produce one comparison artifact instead: the current state plus 2-3 labeled variants (A/B/C) of the same content in the same frame, and let the user pick. For choices with no objective criterion (typeface, accent color, logo), skip the nudging entirely and start with a specimen sheet: up to 5 candidates, each a labeled half-page block of identical title-plus-paragraph content. One round of "pick one" converges where five rounds of "try again" do not; after the pick, apply it everywhere and rebuild affected demos in the same round.

---

## When not to use this skill

- User explicitly wants Material / Fluent / Tailwind default - different design language
- Need dark / cyberpunk / futurist aesthetic (this is deliberately anti-future)
- Need saturated multi-color (this has one accent)
- Need cartoon / animation / illustration style (this is editorial)
- Web dynamic app UI (this is for print / static documents)
