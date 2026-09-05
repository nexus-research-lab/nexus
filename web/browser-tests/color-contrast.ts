// INPUT: A visible control on flat, unfiltered fixture surfaces and its rendered placeholder.
// OUTPUT: Browser-composited foreground/background contrast without glyph antialiasing.
// POS: Visual QA measurement; rejects images, filters and group opacity instead of guessing their paint.

import type { Locator } from "@playwright/test";

export async function measurePlaceholderContrast(control: Locator) {
  return control.evaluate((element) => {
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 1;
    const context = canvas.getContext("2d", { willReadFrequently: true })!;
    const layers: string[] = [];
    let opaque = false;
    for (let ancestor: Element | null = element; ancestor; ancestor = ancestor.parentElement) {
      const style = getComputedStyle(ancestor);
      if (style.backgroundImage !== "none" || Number(style.opacity) !== 1 || style.filter !== "none") {
        throw new Error("Contrast fixture requires flat, unfiltered surfaces with no group opacity");
      }
      layers.unshift(style.backgroundColor);
      context.clearRect(0, 0, 1, 1);
      context.fillStyle = style.backgroundColor;
      context.fillRect(0, 0, 1, 1);
      if (context.getImageData(0, 0, 1, 1).data[3] === 255) {
        opaque = true;
        break;
      }
    }
    if (!opaque) throw new Error("Contrast fixture needs an opaque theme surface");
    context.clearRect(0, 0, 1, 1);
    for (const layer of layers) {
      context.fillStyle = layer;
      context.fillRect(0, 0, 1, 1);
    }
    const background = Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3);
    const placeholder = getComputedStyle(element, "::placeholder");
    context.globalAlpha = Number(placeholder.opacity);
    context.fillStyle = placeholder.color;
    context.fillRect(0, 0, 1, 1);
    const foreground = Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3);
    const luminance = (rgb: number[]) => rgb.reduce((sum, channel, index) => {
      const value = channel / 255;
      return sum + [0.2126, 0.7152, 0.0722][index]!
        * (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4);
    }, 0);
    const values = [luminance(foreground), luminance(background)].sort((a, b) => a - b);
    return {
      ratio: (values[1]! + 0.05) / (values[0]! + 0.05),
      foreground,
      background,
      placeholderShown: element.matches(":placeholder-shown"),
    };
  });
}
