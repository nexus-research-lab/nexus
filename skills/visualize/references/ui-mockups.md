# UI mockups

- Reproduce the requested product surface, not an entire decorative landing page. Omit browser chrome, fake sidebars, duplicate titles, and ornamental hero areas unless they are the subject.
- Use a clear reading order: compact controls, primary content, then secondary detail. Prefer dividers and whitespace over nested cards.
- Use CSS Grid for comparable metrics and Flexbox for compact controls. Keep labels and values aligned to common axes.
- Use one-pixel `--nexus-border` boundaries, `--nexus-surface` for restrained grouping, and 8px or 12px radii. Avoid gradients, glass, glow, and heavy shadows.
- Empty, loading, selected, warning, and error states must remain distinguishable in both light and dark themes.
- On narrow widths, reflow columns and allow tables or timelines to scroll only when their data cannot remain legible otherwise.
