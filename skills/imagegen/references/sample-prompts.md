# Sample Prompts

这些示例对齐 Codex imagegen sample 的 use-case taxonomy，但已经改成 Nexus 的 prompt-only 参考。不要把示例里的每一项都机械套到所有请求里；用户 prompt 已经具体时，只做结构化和消歧。

## Product Mockup

```text
Use case: product-mockup
Asset type: landing page hero
Primary request: a minimal hero image of a ceramic coffee mug
Style/medium: clean product photography
Composition/framing: wide composition with usable negative space for page copy if needed
Lighting/mood: soft studio lighting
Materials/textures: matte ceramic, subtle stone surface
Constraints: no logos, no text, no watermark
```

## UI Mockup

```text
Use case: ui-mockup
Asset type: mobile app screen
Primary request: mobile app home screen for a local farmers market with vendors and daily specials
Style/medium: realistic product UI, not concept art
Composition/framing: clean vertical mobile layout with clear hierarchy
Constraints: practical layout, clear typography, no logos, no watermark
Avoid: decorative clutter, fake unreadable microtext
```

## Infographic

```text
Use case: infographic-diagram
Primary request: detailed infographic of an automatic coffee machine flow
Scene/backdrop: clean, light neutral background
Subject: bean hopper -> grinder -> brew group -> boiler -> water tank -> drip tray
Style/medium: clean infographic with clear callouts and arrows
Composition/framing: vertical poster layout, top-to-bottom flow
Text (verbatim): "Bean Hopper", "Grinder", "Brew Group", "Boiler", "Water Tank", "Drip Tray"
Constraints: clear labels, strong contrast, no logos, no watermark
```

## Scientific Educational

```text
Use case: scientific-educational
Primary request: biology diagram titled "Cellular Respiration at a Glance" for high school students
Scene/backdrop: clean white classroom handout background
Subject: glucose turns into energy inside a cell; include glycolysis, Krebs cycle, and electron transport chain
Style/medium: flat scientific diagram with consistent icons, arrows, and readable labels
Composition/framing: landscape slide-style layout with clear hierarchy and generous whitespace
Text (verbatim): "Cellular Respiration at a Glance", "Glucose", "Pyruvate", "ATP", "NADH", "FADH2", "CO2", "O2", "H2O"
Constraints: scientifically plausible, avoid tiny text, no extra decoration, no watermark
```

## Ads Marketing

```text
Use case: ads-marketing
Asset type: social media campaign image
Primary request: campaign image for a streetwear brand called Thread
Subject: group of friends hanging out together in a stylish urban setting
Style/medium: polished youth streetwear campaign photography
Composition/framing: vertical ad layout with natural poses and integrated headline space
Lighting/mood: contemporary, energetic, tasteful
Text (verbatim): "Yours to Create."
Constraints: render the tagline exactly once, clean legible typography, no extra text, no watermarks, no unrelated logos
```

## Website Hero Background

```text
Use case: stylized-concept
Asset type: landing page hero background
Primary request: minimal abstract background with subtle depth and soft texture
Style/medium: matte illustration / soft-rendered abstract background
Composition/framing: wide composition with usable negative space for page copy
Lighting/mood: gentle studio glow
Color palette: restrained neutral palette
Constraints: no text, no logos, no watermark
```

## Game UI Icon

```text
Use case: stylized-concept
Asset type: game UI icon
Primary request: round shield icon with a subtle rune pattern
Style/medium: painted game UI icon
Composition/framing: centered icon, generous padding, clear silhouette
Constraints: no text, no background scene elements, no logos, no watermark
```

## Transparent Sticker Source

```text
Use case: background-extraction
Asset type: transparent sticker source
Primary request: a cheerful mascot sticker waving
Style/medium: clean sticker illustration with crisp outline
Composition/framing: full subject visible, generous padding
Scene/backdrop: perfectly flat solid #00ff00 chroma-key background for background removal
Constraints: background must be one uniform color with no shadows, gradients, texture, floor plane, reflections, or lighting variation; do not use #00ff00 anywhere in the subject
Avoid: cast shadow, contact shadow, reflection, watermark, random text
```

## Precise Object Edit

```text
Use case: precise-object-edit
Asset type: edited image
Primary request: replace only the background with a warm sunset gradient
Input images: Image 1 is the edit target
Constraints: change only the background; keep the subject, edges, pose, camera angle, and readable text unchanged
Avoid: changing unrelated objects, warped geometry, extra text, watermark
```

## Text Localization

```text
Use case: text-localization
Asset type: localized poster image
Primary request: replace the existing headline with the provided Chinese headline
Input images: Image 1 is the edit target
Text (verbatim): "夏日新品上市"
Constraints: change only the headline text; preserve layout, typography style, spacing, background, colors, and all non-target elements
Avoid: extra words, misspelled text, reflowed layout, watermark
```
