// INPUT: Visible input text on painted App surfaces, including gradients and backdrop blur.
// OUTPUT: Worst text contrast across the rendered input content box, excluding glyph antialiasing.
// POS: Browser screenshot measurement; temporarily hides text and restores the exact input styles.

import type { Locator } from "@playwright/test";

export async function measurePaintedInputContrast(input: Locator, pseudo: "::placeholder" | null = null) {
  const original = await input.evaluate((element, pseudo) => {
    const style = getComputedStyle(element);
    const text = getComputedStyle(element, pseudo);
    for (let ancestor: Element | null = element; ancestor; ancestor = ancestor.parentElement) {
      const paint = getComputedStyle(ancestor);
      if (Number(paint.opacity) !== 1 || paint.filter !== "none") {
        throw new Error("Input contrast requires settled, unfiltered text");
      }
    }
    return {
      color: text.color,
      opacity: Number(text.opacity),
      left: parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft),
      right: parseFloat(style.borderRightWidth) + parseFloat(style.paddingRight),
      top: parseFloat(style.borderTopWidth) + parseFloat(style.paddingTop),
      bottom: parseFloat(style.borderBottomWidth) + parseFloat(style.paddingBottom),
    };
  }, pseudo);
  const rule = await input.page().addStyleTag({ content: "[data-contrast-probe], [data-contrast-probe]::placeholder { color: transparent !important; text-shadow: none !important; }" });
  try {
    await input.evaluate((element) => element.setAttribute("data-contrast-probe", ""));
    const screenshot = await input.screenshot({ animations: "disabled", caret: "hide", scale: "css" });
    return await input.evaluate(async (_, { data, paint }) => {
      const image = new Image();
      image.src = `data:image/png;base64,${data}`;
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.width;
      canvas.height = image.height;
      const context = canvas.getContext("2d", { willReadFrequently: true })!;
      context.drawImage(image, 0, 0);
      const pixels = context.getImageData(0, 0, image.width, image.height).data;
      context.globalAlpha = paint.opacity;
      context.fillStyle = paint.color;
      context.fillRect(0, 0, image.width, image.height);
      const foreground = context.getImageData(0, 0, image.width, image.height).data;
      const luminance = (data: Uint8ClampedArray, offset: number) => [0.2126, 0.7152, 0.0722].reduce((sum, weight, channel) => {
        const value = data[offset + channel]! / 255;
        return sum + weight * (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4);
      }, 0);
      let ratio = Infinity;
      for (let y = Math.ceil(paint.top); y < image.height - paint.bottom; y++) {
        for (let x = Math.ceil(paint.left); x < image.width - paint.right; x++) {
          const offset = (y * image.width + x) * 4;
          const background = luminance(pixels, offset);
          const text = luminance(foreground, offset);
          ratio = Math.min(ratio, (Math.max(text, background) + 0.05) / (Math.min(text, background) + 0.05));
        }
      }
      if (!Number.isFinite(ratio)) throw new Error("Input contrast requires a nonempty content box");
      return { ratio };
    }, { data: screenshot.toString("base64"), paint: original });
  } finally {
    await input.evaluate((element) => element.removeAttribute("data-contrast-probe"));
    await rule.evaluate((element) => element.parentNode?.removeChild(element));
  }
}
