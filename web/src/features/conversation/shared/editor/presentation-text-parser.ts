import type {
  PresentationParagraph,
  PresentationTextRun,
} from "./presentation-preview-model";
import { readFillColor } from "./presentation-shape-style";
import {
  childrenByLocalName,
  emuToPixel,
  firstChildByLocalName,
  firstDescendantByLocalName,
} from "./presentation-xml-utils";

export function parseTextBody(textBody: Element | null, shapeWidth: number): PresentationParagraph[] {
  if (!textBody) {
    return [];
  }

  const listStyle = firstChildByLocalName(textBody, "lstStyle");

  return childrenByLocalName(textBody, "p")
    .map((paragraph) => {
      const paragraphProperties = firstChildByLocalName(paragraph, "pPr");
      const listParagraphProperties = readListParagraphProperties(listStyle, paragraphProperties);
      const defaultRunProperties = [
        firstChildByLocalName(paragraphProperties, "defRPr"),
        firstChildByLocalName(listParagraphProperties, "defRPr"),
        firstChildByLocalName(paragraph, "endParaRPr"),
      ];
      const align = readParagraphAlign(paragraphProperties);
      const defaultFontSize = readFontSizeFromCandidates(defaultRunProperties, shapeWidth);
      const runs = childrenByLocalName(paragraph, "r")
        .map((run) => parseTextRun(run, defaultRunProperties, shapeWidth, defaultFontSize))
        .filter((run): run is PresentationTextRun => !!run && run.text.length > 0);
      const fallbackText = runs.length === 0 ? firstDescendantByLocalName(paragraph, "t")?.textContent || "" : "";
      const textRuns = runs.length > 0
        ? runs
        : [{
          color: readFillColorFromCandidates(defaultRunProperties) || "#111827",
          fontFace: readFontFaceFromCandidates(defaultRunProperties),
          fontSize: defaultFontSize,
          text: fallbackText,
        }];
      const text = textRuns.map((run) => run.text).join("");
      const paragraphFontSize = textRuns[0]?.fontSize || defaultFontSize;

      return {
        align,
        bullet: readParagraphBullet(paragraphProperties),
        bulletIndent: readParagraphBulletIndent(paragraphProperties, paragraphFontSize),
        fontSize: paragraphFontSize,
        lineHeight: readParagraphLineHeight(paragraphProperties, listParagraphProperties),
        runs: textRuns,
        text,
      };
    })
    .filter((paragraph) => paragraph.text.trim().length > 0);
}

function parseTextRun(
  run: Element,
  defaultRunProperties: Array<Element | null>,
  shapeWidth: number,
  defaultFontSize: number,
): PresentationTextRun | null {
  const text = firstDescendantByLocalName(run, "t")?.textContent || "";
  if (!text) {
    return null;
  }

  const runProperties = firstChildByLocalName(run, "rPr");
  const runPropertyChain = [runProperties, ...defaultRunProperties];
  return {
    bold: readBooleanAttributeFromCandidates(runPropertyChain, "b", false),
    color: readFillColorFromCandidates(runPropertyChain) || "#111827",
    fontFace: readFontFaceFromCandidates(runPropertyChain),
    fontSize: readFontSizeFromCandidates(runPropertyChain, shapeWidth, defaultFontSize),
    italic: readBooleanAttributeFromCandidates(runPropertyChain, "i", false),
    text,
  };
}

function readListParagraphProperties(listStyle: Element | null, paragraphProperties: Element | null): Element | null {
  const level = Math.max(Number(paragraphProperties?.getAttribute("lvl") || 0), 0);
  return firstChildByLocalName(listStyle, `lvl${level + 1}pPr`)
    || firstChildByLocalName(listStyle, "defPPr");
}

function readBooleanAttributeFromCandidates(
  elements: Array<Element | null>,
  attribute: string,
  fallback: boolean,
): boolean {
  for (const element of elements) {
    const value = element?.getAttribute(attribute);
    if (value === "1" || value === "true") {
      return true;
    }
    if (value === "0" || value === "false") {
      return false;
    }
  }
  return fallback;
}

function readFontFace(runProperties: Element | null): string | undefined {
  return firstDescendantByLocalName(runProperties, "ea")?.getAttribute("typeface")
    || firstDescendantByLocalName(runProperties, "latin")?.getAttribute("typeface")
    || undefined;
}

function readFontFaceFromCandidates(elements: Array<Element | null>): string | undefined {
  for (const element of elements) {
    const fontFace = readFontFace(element);
    if (fontFace) {
      return fontFace;
    }
  }
  return undefined;
}

function readFillColorFromCandidates(elements: Array<Element | null>): string | undefined {
  for (const element of elements) {
    const color = readFillColor(element);
    if (color) {
      return color;
    }
  }
  return undefined;
}

function readParagraphBullet(paragraphProperties: Element | null): string | undefined {
  if (!paragraphProperties || firstChildByLocalName(paragraphProperties, "buNone")) {
    return undefined;
  }
  return firstChildByLocalName(paragraphProperties, "buChar")?.getAttribute("char") || undefined;
}

function readParagraphBulletIndent(paragraphProperties: Element | null, fontSize: number): number {
  const margin = Number(paragraphProperties?.getAttribute("marL") || 0);
  if (margin > 0) {
    return Math.max(emuToPixel(margin), fontSize * 1.2);
  }
  return fontSize * 1.35;
}

function readParagraphLineHeight(
  paragraphProperties: Element | null,
  listParagraphProperties?: Element | null,
): number {
  const lineSpacing = firstDescendantByLocalName(paragraphProperties, "lnSpc")
    || firstDescendantByLocalName(listParagraphProperties || null, "lnSpc");
  const spacingPercent = Number(firstChildByLocalName(lineSpacing, "spcPct")?.getAttribute("val") || 0);
  if (spacingPercent > 0) {
    return Math.max(spacingPercent / 100000, 1);
  }
  return 1.18;
}

function readParagraphAlign(paragraphProperties: Element | null): PresentationParagraph["align"] {
  const align = paragraphProperties?.getAttribute("algn");
  if (align === "ctr") {
    return "center";
  }
  if (align === "r") {
    return "right";
  }
  return "left";
}

function readFontSizeFromCandidates(
  elements: Array<Element | null>,
  shapeWidth: number,
  fallbackSize?: number,
): number {
  for (const element of elements) {
    const size = Number(element?.getAttribute("sz") || 0);
    if (size > 0) {
      return Math.max((size / 100) * (96 / 72), 8);
    }
  }
  if (fallbackSize) {
    return fallbackSize;
  }
  return Math.max(Math.min(shapeWidth / 16, 24), 13);
}
