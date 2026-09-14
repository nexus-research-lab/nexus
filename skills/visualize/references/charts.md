# Charts

For controls or dynamic state, also read [interaction.md](interaction.md).

- Use SVG or native DOM for small charts. Use Chart.js only when axes, tooltips, or multiple dynamic series justify it.
- Wrap each canvas in a `position:relative` container with an explicit height. Do not set CSS height on canvas. Use `responsive:true` and `maintainAspectRatio:false`.
- Canvas cannot resolve CSS variables. Read `--nexus-chart-1` through `--nexus-chart-5` with `getComputedStyle(document.documentElement).getPropertyValue(name).trim()`.
- Assigning `canvas.width` or `canvas.height` clears the bitmap and resets the context. Set the backing size only during initialization or a real resize, never inside draw or coordinate helpers.
- Give every canvas a unique id. Keep the chart instance and guard initialization so CDN `onload` plus an immediate fallback cannot create it twice.
- Load established UMD builds over HTTPS. Put the library script before the initializer, use `onload` to call a named init function, and also call it when the global already exists.
- Controls must update chart data and call `chart.update()`. Disable library legends when a compact HTML legend communicates values more clearly.
- Round displayed values consistently, label axes and units, and pad plot ranges so points and labels are not clipped.
