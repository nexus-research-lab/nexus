# Diagrams

- Prefer one responsive SVG with `width="100%"` and a complete `viewBox`. Put `defs` and arrow markers before visible nodes so streaming connectors are valid.
- Choose the structure that matches the idea: flow for sequence, hierarchy for ownership, cycle for feedback, matrix for two dimensions, timeline for change, or side-by-side for comparison.
- Keep node titles to five words when possible and at most four full-size nodes per row. Put detail in surrounding prose or an interactive inspector.
- Calculate the `viewBox` from the content bounds on all four sides plus modest padding, including labels and arrowheads. Avoid unused space above the diagram. Account for `text-anchor` direction.
- Connect edges from node boundaries, use a shared marker, and verify no edge crosses unrelated nodes or text.
- Use neutral structure plus no more than two categorical chart colors. Encode status with text or shape as well as color.
- For interactive diagrams, mutate classes and SVG attributes on existing elements instead of rebuilding the SVG.

When `diagram-design` is installed, use it for a persistent, downloadable single-file HTML diagram rather than an inline conversational widget.

## Composition and scale

- Prefer HTML for node captions, metrics, and the detail inspector when SVG scaling would make text too large on wide screens or unreadable on narrow screens. Keep text embedded in SVG when its position is part of the diagram's meaning.
- Bound the drawing's growth on wide screens; use the actual display size to judge type and spacing. On narrow screens, reflow the sequence or change orientation instead of shrinking a dense diagram until labels become illegible.
- Reserve a separate lane for edge labels. Size gaps for the label as well as the arrow, and keep both clear of node stacks and captions.
- Distinguish node name, primary value, and secondary metadata through spacing and muted color rather than making all three equally prominent.
- Give selectable nodes a visible selected state, hover and focus feedback, and keyboard activation. Keep details in one inspector and update it together with the selected visual.

## Example: layered network explanation

Use a compact sequence of stages with aligned captions and operation labels between them. Keep dimensions under each stage and secondary quantities in muted text. Selecting a stage updates one inspector below the sequence and highlights the corresponding region or relationship in the drawing. If stack depth is schematic, label the channel count explicitly rather than suggesting that drawn sheets are an exact count. Let the drawing begin near its top content edge; do not reserve a large empty presentation area.
