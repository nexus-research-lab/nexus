# SVG and Canvas art

For controls or dynamic state, also read [interaction.md](interaction.md).

- Prefer SVG for illustrations and finite diagrams; use Canvas for continuous animation, dense particles, or pixel-level drawing.
- For Canvas, initialize the backing store once per actual size change, scale for `devicePixelRatio` once, and keep coordinate conversion separate from drawing.
- Resolve Nexus color variables to concrete values before assigning `fillStyle`, `strokeStyle`, shadows, or gradients.
- Run animation through `requestAnimationFrame`, cap particle or object counts, and pause or simplify when `prefers-reduced-motion` is enabled.
- Keep controls and captions as accessible HTML outside Canvas. Do not make essential meaning depend on pixels alone.
- Keep the outer surface transparent and let the artwork, not decorative containers, carry the composition.
