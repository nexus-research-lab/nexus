// INPUT: Parsed slide geometry, repeated paragraphs/runs and thumbnail mode.
// OUTPUT: Stable text sequences and matching full/thumbnail content geometry.
// POS: Offline SVG DOM regression; no PowerPoint decoding or pixel rendering assertion.
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PresentationSlideCanvas } from "./presentation-slide-canvas";
import type { PresentationParagraph, PresentationSlide } from "./presentation-preview-model";

const paragraph: PresentationParagraph = {
  text: "RepeatRepeat", bullet: "•", bulletIndent: 24, fontSize: 20, lineHeight: 1.2, align: "center",
  runs: [0, 1].map(() => ({ text: "Repeat", fontSize: 20, color: "#123456", fontFace: "Test Font", bold: true })),
};
const slide: PresentationSlide = {
  id: "slide", title: "Preview", background: "#ffffff", width: 1280, height: 720,
  elements: [{ type: "shape", id: "shape", geometry: "roundRect", x: 20, y: 30, width: 300, height: 200,
    paragraphs: [paragraph, paragraph], strokeWidth: 2, stroke: "#654321", fill: "#eee", textAnchor: "center" }],
};
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("keeps repeated paragraphs and runs through updates without duplicate key warnings", () => {
  const errors = vi.spyOn(console, "error").mockImplementation(() => {});
  const view = render(<PresentationSlideCanvas slide={slide} />);
  expect(screen.getAllByText("Repeat")).toHaveLength(4);
  const shape = slide.elements[0];
  if (shape.type !== "shape") throw new Error("Expected shape fixture");
  view.rerender(<PresentationSlideCanvas slide={{ ...slide, elements: [{ ...shape, paragraphs: [paragraph] }] }} />);
  expect(screen.getAllByText("Repeat")).toHaveLength(2);
  expect(errors.mock.calls.filter((args) => args.some((value) => String(value).includes("same key")))).toEqual([]);
});

it("scales the same content for thumbnails and preserves source text/shape styling", () => {
  const view = render(<PresentationSlideCanvas slide={slide} />);
  const svg = screen.getByRole("img", { name: "Preview" });
  const content = svg.innerHTML;
  expect(svg.getAttribute("viewBox")).toBe("0 0 1280 720");
  expect((screen.getAllByText("Repeat")[0] as HTMLElement).style.fontSize).toBe("20px");
  expect(svg.querySelector("foreignObject > div")?.getAttribute("style")).toContain("justify-content: center");
  expect(svg.querySelector("g > rect")?.getAttribute("rx")).toBe("14");
  view.rerender(<PresentationSlideCanvas slide={slide} thumbnail />);
  expect(svg.innerHTML).toBe(content);
});
