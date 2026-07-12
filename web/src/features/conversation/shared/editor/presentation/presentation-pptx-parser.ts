import type JSZip from "jszip";

import {
  DEFAULT_SLIDE_HEIGHT_EMU,
  DEFAULT_SLIDE_WIDTH_EMU,
  SLIDE_LAYOUT_RELATIONSHIP_TYPE,
  SLIDE_MASTER_RELATIONSHIP_TYPE,
  type PresentationElement,
  type PresentationParseResult,
  type PresentationPlaceholderStyle,
  type PresentationRelationship,
  type PresentationSlide,
} from "./presentation-preview-model";
import { readSlideBackground } from "./presentation-shape-style";
import {
  createEmptyPresentationShapeTreeResult,
  parsePresentationShapeTree,
  type PresentationShapeTreeResult,
} from "./parser/presentation-shape-tree-parser";
import {
  descendantsByLocalName,
  emuToPixel,
  firstDescendantByLocalName,
  parseXml,
  readRelationships,
  readZipText,
  relationshipAttribute,
  resolveRelationshipTarget,
  revokeObjectUrls,
} from "./presentation-xml-utils";

interface PresentationSlideParts {
  layoutPart: PresentationPart | null;
  masterPart: PresentationPart | null;
  slideDoc: Document;
  slideRelationships: Record<string, PresentationRelationship>;
}

interface PresentationPart {
  background?: string;
  elements: PresentationElement[];
  placeholderStyles: Map<string, PresentationPlaceholderStyle>;
}

interface SlideSize {
  height: number;
  width: number;
}

export async function parsePptx(
  buffer: ArrayBuffer,
): Promise<PresentationParseResult> {
  const { default: JSZipConstructor } = await import("jszip");
  const zip = await JSZipConstructor.loadAsync(buffer);
  return new PresentationPackageParser(zip).parse();
}

/** 包解析器统一持有共享 Part 缓存和 Object URL 生命周期。 */
class PresentationPackageParser {
  private readonly objectUrls: string[] = [];
  private readonly partCache = new Map<
    string,
    Promise<PresentationPart | null>
  >();

  constructor(private readonly zip: JSZip) {}

  async parse(): Promise<PresentationParseResult> {
    try {
      const presentationXml = await readZipText(
        this.zip,
        "ppt/presentation.xml",
      );
      const presentationDoc = parseXml(presentationXml);
      const presentationRelationships = await readRelationships(
        this.zip,
        "ppt/presentation.xml",
      );
      const size = readSlideSize(presentationDoc);
      const declaredPaths = readSlidePaths(
        presentationDoc,
        presentationRelationships,
      );
      const slidePaths = declaredPaths.length > 0
        ? declaredPaths
        : fallbackSlidePaths(this.zip);
      if (slidePaths.length === 0) {
        throw new Error("pptx 文件中没有可预览的幻灯片");
      }
      const slides: PresentationSlide[] = [];
      for (const [index, slidePath] of slidePaths.entries()) {
        slides.push(await this.parseSlide(slidePath, index, size));
      }
      return { objectUrls: this.objectUrls, slides };
    } catch (error) {
      revokeObjectUrls(this.objectUrls);
      throw error;
    }
  }

  private async parseSlide(
    slidePath: string,
    index: number,
    size: SlideSize,
  ): Promise<PresentationSlide> {
    const parts = await this.readSlideParts(slidePath);
    const inheritedPlaceholders = mergePlaceholderStyles(
      parts.masterPart?.placeholderStyles,
      parts.layoutPart?.placeholderStyles,
    );
    const slideResult = await this.parseSlideShapeTree(
      slidePath,
      parts,
      index,
      inheritedPlaceholders,
    );
    return {
      background: resolveSlideBackground(parts),
      elements: collectSlideElements(parts, slideResult),
      height: size.height,
      id: `slide-${index + 1}`,
      title: readSlideTitle(slideResult.elements, index),
      width: size.width,
    };
  }

  private async readSlideParts(
    slidePath: string,
  ): Promise<PresentationSlideParts> {
    const slideXml = await readZipText(this.zip, slidePath);
    const slideDoc = parseXml(slideXml);
    const slideRelationships = await readRelationships(this.zip, slidePath);
    const layoutPath = resolveRelatedPartPath(
      slidePath,
      slideRelationships,
      SLIDE_LAYOUT_RELATIONSHIP_TYPE,
    );
    const layoutRelationships = await readOptionalRelationships(
      this.zip,
      layoutPath,
    );
    const masterPath = resolveRelatedPartPath(
      layoutPath ?? "",
      layoutRelationships,
      SLIDE_MASTER_RELATIONSHIP_TYPE,
    );
    const masterPart = await this.parseOptionalPart(masterPath);
    const layoutPart = await this.parseOptionalPart(
      layoutPath,
      masterPart?.placeholderStyles,
    );
    return {
      layoutPart,
      masterPart,
      slideDoc,
      slideRelationships,
    };
  }

  private async parseSlideShapeTree(
    slidePath: string,
    parts: PresentationSlideParts,
    index: number,
    inheritedPlaceholders: Map<string, PresentationPlaceholderStyle>,
  ): Promise<PresentationShapeTreeResult> {
    const shapeTree = firstDescendantByLocalName(parts.slideDoc, "spTree");
    if (!shapeTree) {
      return createEmptyPresentationShapeTreeResult();
    }
    return parsePresentationShapeTree({
      context: {
        elementIndex: 0,
        fallbackPlaceholders: inheritedPlaceholders,
        idPrefix: `slide-${index + 1}`,
        includePlaceholderShapes: true,
      },
      objectUrls: this.objectUrls,
      relationships: parts.slideRelationships,
      shapeTree,
      sourcePath: slidePath,
      zip: this.zip,
    });
  }

  private parseOptionalPart(
    partPath: string | null,
    fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>,
  ): Promise<PresentationPart | null> {
    if (!partPath) {
      return Promise.resolve(null);
    }
    const cached = this.partCache.get(partPath);
    if (cached) {
      return cached;
    }
    const parsing = this.parsePart(partPath, fallbackPlaceholders);
    this.partCache.set(partPath, parsing);
    return parsing;
  }

  private async parsePart(
    partPath: string,
    fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>,
  ): Promise<PresentationPart | null> {
    if (!this.zip.file(partPath)) {
      return null;
    }
    const partXml = await readZipText(this.zip, partPath);
    const partDoc = parseXml(partXml);
    const relationships = await readRelationships(this.zip, partPath);
    const shapeTree = firstDescendantByLocalName(partDoc, "spTree");
    const result = shapeTree
      ? await parsePresentationShapeTree({
        context: {
          elementIndex: 0,
          fallbackPlaceholders,
          idPrefix: partPath.replace(/[^a-z0-9]+/gi, "-"),
          includePlaceholderShapes: false,
        },
        objectUrls: this.objectUrls,
        relationships,
        shapeTree,
        sourcePath: partPath,
        zip: this.zip,
      })
      : createEmptyPresentationShapeTreeResult();
    return {
      background: readSlideBackground(partDoc),
      elements: result.elements,
      placeholderStyles: result.placeholderStyles,
    };
  }
}

function readSlideSize(presentationDoc: Document): SlideSize {
  const slideSize = firstDescendantByLocalName(presentationDoc, "sldSz");
  const widthEmu = Number(
    slideSize?.getAttribute("cx") || DEFAULT_SLIDE_WIDTH_EMU,
  );
  const heightEmu = Number(
    slideSize?.getAttribute("cy") || DEFAULT_SLIDE_HEIGHT_EMU,
  );
  return {
    height: Math.max(emuToPixel(heightEmu), 1),
    width: Math.max(emuToPixel(widthEmu), 1),
  };
}

function readSlidePaths(
  presentationDoc: Document,
  relationships: Record<string, PresentationRelationship>,
): string[] {
  return descendantsByLocalName(presentationDoc, "sldId")
    .map((slideId) => {
      const relationshipId = relationshipAttribute(slideId, "id");
      const relationship = relationshipId
        ? relationships[relationshipId]
        : undefined;
      return relationship
        ? resolveRelationshipTarget(
          "ppt/presentation.xml",
          relationship.target,
        )
        : null;
    })
    .filter((path): path is string => path !== null);
}

function fallbackSlidePaths(zip: JSZip): string[] {
  return Object.keys(zip.files)
    .filter((path) => /^ppt\/slides\/slide\d+\.xml$/i.test(path))
    .sort((left, right) => (
      readSlidePathIndex(left) - readSlidePathIndex(right)
    ));
}

function readSlidePathIndex(path: string): number {
  return Number(path.match(/slide(\d+)\.xml$/i)?.[1] || 0);
}

async function readOptionalRelationships(
  zip: JSZip,
  partPath: string | null,
): Promise<Record<string, PresentationRelationship>> {
  return partPath ? readRelationships(zip, partPath) : {};
}

function resolveSlideBackground(parts: PresentationSlideParts): string {
  return readSlideBackground(parts.slideDoc)
    ?? parts.layoutPart?.background
    ?? parts.masterPart?.background
    ?? "#ffffff";
}

function collectSlideElements(
  parts: PresentationSlideParts,
  slideResult: PresentationShapeTreeResult,
): PresentationElement[] {
  return [
    ...readPartElements(parts.masterPart),
    ...readPartElements(parts.layoutPart),
    ...slideResult.elements,
  ];
}

function readPartElements(part: PresentationPart | null): PresentationElement[] {
  return part?.elements ?? [];
}

function readSlideTitle(
  elements: PresentationElement[],
  index: number,
): string {
  const firstText = elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);
  return firstText ?? `幻灯片 ${index + 1}`;
}

function resolveRelatedPartPath(
  sourcePath: string,
  relationships: Record<string, PresentationRelationship>,
  relationshipType: string,
): string | null {
  const relationship = Object.values(relationships).find(
    (candidate) => candidate.type === relationshipType,
  );
  if (!relationship || relationship.targetMode === "External") {
    return null;
  }
  return resolveRelationshipTarget(sourcePath, relationship.target);
}

function mergePlaceholderStyles(
  base?: Map<string, PresentationPlaceholderStyle>,
  override?: Map<string, PresentationPlaceholderStyle>,
): Map<string, PresentationPlaceholderStyle> {
  return new Map([
    ...(base?.entries() ?? []),
    ...(override?.entries() ?? []),
  ]);
}
