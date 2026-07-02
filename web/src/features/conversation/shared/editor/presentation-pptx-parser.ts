import type JSZip from "jszip";

import {
  DEFAULT_SLIDE_HEIGHT_EMU,
  DEFAULT_SLIDE_WIDTH_EMU,
  MIN_BACKGROUND_LIKE_SHAPE_SIZE,
  MIN_DECORATION_SHAPE_SIZE,
  SLIDE_LAYOUT_RELATIONSHIP_TYPE,
  SLIDE_MASTER_RELATIONSHIP_TYPE,
  type PresentationElement,
  type PresentationGroupTransform,
  type PresentationImageElement,
  type PresentationParagraph,
  type PresentationParseResult,
  type PresentationPart,
  type PresentationPlaceholderStyle,
  type PresentationRelationship,
  type PresentationShapeElement,
  type PresentationShapeGeometry,
  type PresentationShapeTreeContext,
  type PresentationShapeTreeResult,
  type PresentationSlide,
} from "./presentation-preview-model";
import {
  apply_group_transform_to_element,
  map_group_placeholder_styles,
  read_fill_color,
  read_group_transform,
  read_shape_geometry,
  read_slide_background,
  read_stroke_color,
  read_stroke_width,
  read_transform,
} from "./presentation-shape-style";
import { parse_text_body } from "./presentation-text-parser";
import {
  descendants_by_local_name,
  emu_to_pixel,
  first_child_by_local_name,
  first_descendant_by_local_name,
  parse_xml,
  read_relationships,
  read_zip_text,
  relationship_attribute,
  resolve_relationship_target,
  revoke_object_urls,
} from "./presentation-xml-utils";

export async function parse_pptx(buffer: ArrayBuffer): Promise<PresentationParseResult> {
  const { default: JSZipConstructor } = await import("jszip");
  const zip = await JSZipConstructor.loadAsync(buffer);
  const objectUrls: string[] = [];

  try {
    const presentationXml = await read_zip_text(zip, "ppt/presentation.xml");
    const presentationDoc = parse_xml(presentationXml);
    const presentationRels = await read_relationships(zip, "ppt/presentation.xml");
    const { height, width } = readSlideSize(presentationDoc);
    const slidePaths = readSlidePaths(presentationDoc, presentationRels);
    const resolvedSlidePaths = slidePaths.length > 0 ? slidePaths : fallbackSlidePaths(zip);

    if (resolvedSlidePaths.length === 0) {
      throw new Error("pptx 文件中没有可预览的幻灯片");
    }

    const slides: PresentationSlide[] = [];
    for (let index = 0; index < resolvedSlidePaths.length; index += 1) {
      const slide = await parseSlide(zip, resolvedSlidePaths[index], index, width, height, objectUrls);
      slides.push(slide);
    }

    return { object_urls: objectUrls, slides };
  } catch (error) {
    revoke_object_urls(objectUrls);
    throw error;
  }
}

function readSlideSize(presentationDoc: Document): { height: number; width: number } {
  const slideSize = first_descendant_by_local_name(presentationDoc, "sldSz");
  const widthEmu = Number(slideSize?.getAttribute("cx") || DEFAULT_SLIDE_WIDTH_EMU);
  const heightEmu = Number(slideSize?.getAttribute("cy") || DEFAULT_SLIDE_HEIGHT_EMU);
  return {
    height: Math.max(emu_to_pixel(heightEmu), 1),
    width: Math.max(emu_to_pixel(widthEmu), 1),
  };
}

function readSlidePaths(
  presentationDoc: Document,
  presentationRels: Record<string, PresentationRelationship>,
): string[] {
  return descendants_by_local_name(presentationDoc, "sldId")
    .map((slideId) => {
      const relId = relationship_attribute(slideId, "id");
      const rel = relId ? presentationRels[relId] : undefined;
      return rel ? resolve_relationship_target("ppt/presentation.xml", rel.target) : null;
    })
    .filter((path): path is string => !!path);
}

function fallbackSlidePaths(zip: JSZip): string[] {
  return Object.keys(zip.files)
    .filter((path) => /^ppt\/slides\/slide\d+\.xml$/i.test(path))
    .sort((left, right) => {
      const leftNumber = Number(left.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      const rightNumber = Number(right.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      return leftNumber - rightNumber;
    });
}

async function parseSlide(
  zip: JSZip,
  slidePath: string,
  index: number,
  width: number,
  height: number,
  objectUrls: string[],
): Promise<PresentationSlide> {
  const slideXml = await read_zip_text(zip, slidePath);
  const slideDoc = parse_xml(slideXml);
  const slideRels = await read_relationships(zip, slidePath);
  const layoutPath = resolveRelatedPartPath(slidePath, slideRels, SLIDE_LAYOUT_RELATIONSHIP_TYPE);
  const layoutRels = layoutPath ? await read_relationships(zip, layoutPath) : {};
  const masterPath = layoutPath
    ? resolveRelatedPartPath(layoutPath, layoutRels, SLIDE_MASTER_RELATIONSHIP_TYPE)
    : null;
  const masterPart = masterPath ? await parsePresentationPart(zip, masterPath, objectUrls) : null;
  const layoutPart = layoutPath
    ? await parsePresentationPart(zip, layoutPath, objectUrls, masterPart?.placeholder_styles)
    : null;
  const inheritedPlaceholders = mergePlaceholderStyles(
    masterPart?.placeholder_styles,
    layoutPart?.placeholder_styles,
  );
  const background = read_slide_background(slideDoc)
    || layoutPart?.background
    || masterPart?.background
    || "#ffffff";
  const shapeTree = first_descendant_by_local_name(slideDoc, "spTree");
  const slideResult = shapeTree ? await parseShapeTree(
    zip,
    slidePath,
    slideRels,
    shapeTree,
    objectUrls,
    {
      element_index: 0,
      fallback_placeholders: inheritedPlaceholders,
      id_prefix: `slide-${index + 1}`,
      include_placeholder_shapes: true,
    },
  ) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };
  const elements = [
    ...(masterPart?.elements ?? []),
    ...(layoutPart?.elements ?? []),
    ...slideResult.elements,
  ];
  const firstText = slideResult.elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);

  return {
    background,
    elements,
    height,
    id: `slide-${index + 1}`,
    title: firstText || `幻灯片 ${index + 1}`,
    width,
  };
}

async function parsePresentationPart(
  zip: JSZip,
  partPath: string,
  objectUrls: string[],
  fallbackPlaceholders?: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationPart | null> {
  if (!zip.file(partPath)) {
    return null;
  }

  const partXml = await read_zip_text(zip, partPath);
  const partDoc = parse_xml(partXml);
  const rels = await read_relationships(zip, partPath);
  const shapeTree = first_descendant_by_local_name(partDoc, "spTree");
  const result = shapeTree ? await parseShapeTree(zip, partPath, rels, shapeTree, objectUrls, {
    element_index: 0,
    fallback_placeholders: fallbackPlaceholders,
    id_prefix: partPath.replace(/[^a-z0-9]+/gi, "-"),
    include_placeholder_shapes: false,
  }) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };

  return {
    background: read_slide_background(partDoc),
    elements: result.elements,
    placeholder_styles: result.placeholder_styles,
    rels,
  };
}

function resolveRelatedPartPath(
  sourcePath: string,
  sourceRels: Record<string, PresentationRelationship>,
  relationshipType: string,
): string | null {
  const rel = Object.values(sourceRels).find((relationship) => relationship.type === relationshipType);
  if (!rel || rel.target_mode === "External") {
    return null;
  }
  return resolve_relationship_target(sourcePath, rel.target);
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

async function parseShapeTree(
  zip: JSZip,
  slidePath: string,
  rels: Record<string, PresentationRelationship>,
  shapeTree: Element,
  objectUrls: string[],
  context: PresentationShapeTreeContext,
  groupTransform?: PresentationGroupTransform | null,
): Promise<PresentationShapeTreeResult> {
  const elements: PresentationElement[] = [];
  const placeholderStyles = new Map<string, PresentationPlaceholderStyle>();
  const children = Array.from(shapeTree.children);

  for (const child of children) {
    switch (child.localName) {
      case "cxnSp":
      case "sp": {
        const parsedShape = parseShape(child, `${context.id_prefix}-shape-${context.element_index}`, context);
        context.element_index += 1;
        if (parsedShape.placeholder_style) {
          placeholderStyles.set(parsedShape.placeholder_style.key, parsedShape.placeholder_style);
        }
        if (parsedShape.shape && (!parsedShape.is_placeholder || context.include_placeholder_shapes)) {
          elements.push(parsedShape.shape);
        }
        break;
      }
      case "grpSp": {
        const groupResult = await parseShapeTree(
          zip,
          slidePath,
          rels,
          child,
          objectUrls,
          context,
          read_group_transform(child),
        );
        groupResult.placeholder_styles.forEach((style, key) => {
          placeholderStyles.set(key, style);
        });
        elements.push(...groupResult.elements);
        break;
      }
      case "pic": {
        const image = await parsePicture(
          zip,
          slidePath,
          rels,
          child,
          `${context.id_prefix}-image-${context.element_index}`,
          objectUrls,
        );
        context.element_index += 1;
        if (image) {
          elements.push(image);
        }
        break;
      }
      default:
        break;
    }
  }

  if (!groupTransform) {
    return { elements, placeholder_styles: placeholderStyles };
  }

  return {
    elements: elements.map((element) => apply_group_transform_to_element(element, groupTransform)),
    placeholder_styles: map_group_placeholder_styles(placeholderStyles, groupTransform),
  };
}

function parseShape(
  element: Element,
  id: string,
  context: PresentationShapeTreeContext,
): {
  is_placeholder: boolean;
  placeholder_style: PresentationPlaceholderStyle | null;
  shape: PresentationShapeElement | null;
} {
  const shapeProperties = first_child_by_local_name(element, "spPr");
  const placeholderKey = readPlaceholderKey(element);
  const fallbackPlaceholder = placeholderKey ? context.fallback_placeholders?.get(placeholderKey) : undefined;
  const transform = read_transform(shapeProperties) || fallbackPlaceholder?.transform || null;
  if (!transform) {
    return {
      is_placeholder: !!placeholderKey,
      placeholder_style: null,
      shape: null,
    };
  }

  const textBody = first_child_by_local_name(element, "txBody");
  const paragraphs = parse_text_body(textBody, transform.width);
  const textAnchor = readTextAnchor(first_child_by_local_name(textBody, "bodyPr"));
  const fill = read_fill_color(shapeProperties) || fallbackPlaceholder?.fill;
  const stroke = read_stroke_color(shapeProperties) || fallbackPlaceholder?.stroke;
  const strokeWidth = read_stroke_width(shapeProperties) || fallbackPlaceholder?.stroke_width || 1;
  const geometry = read_shape_geometry(shapeProperties, element.localName === "cxnSp", fallbackPlaceholder?.geometry);
  const placeholderStyle = placeholderKey ? {
    fill,
    geometry,
    key: placeholderKey,
    stroke,
    stroke_width: strokeWidth,
    transform,
  } : null;

  if (shouldSkipShapePreview({ fill, geometry, height: transform.height, paragraphs, stroke, width: transform.width })) {
    return {
      is_placeholder: !!placeholderKey,
      placeholder_style: placeholderStyle,
      shape: null,
    };
  }

  return {
    is_placeholder: !!placeholderKey,
    placeholder_style: placeholderStyle,
    shape: {
      ...transform,
      fill,
      geometry,
      id,
      paragraphs,
      stroke,
      stroke_width: strokeWidth,
      text_anchor: textAnchor,
      type: "shape",
    },
  };
}

function shouldSkipShapePreview({
  fill,
  geometry,
  height,
  paragraphs,
  stroke,
  width,
}: {
  fill?: string;
  geometry: PresentationShapeGeometry;
  height: number;
  paragraphs: PresentationParagraph[];
  stroke?: string;
  width: number;
}): boolean {
  if (geometry === "line") {
    return false;
  }
  if (geometry === "unsupported" && paragraphs.length === 0) {
    return true;
  }
  if (!fill && !stroke && paragraphs.length === 0) {
    return true;
  }

  // 中文注释：PPT 里有些装饰点/图标会以复杂几何降级成小描边矩形。
  // 预览无法高保真还原时，隐藏它比显示误导性的半成品更接近系统预览体验。
  return (
    geometry === "rect" &&
    !fill &&
    !!stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) <= MIN_DECORATION_SHAPE_SIZE
  ) || (
    geometry === "roundRect" &&
    isPlainWhiteFill(fill) &&
    !stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) >= MIN_BACKGROUND_LIKE_SHAPE_SIZE
  );
}

function isPlainWhiteFill(fill?: string): boolean {
  const normalizedFill = fill?.toLowerCase();
  return normalizedFill === "#ffffff" || normalizedFill === "#fff";
}

async function parsePicture(
  zip: JSZip,
  slidePath: string,
  rels: Record<string, PresentationRelationship>,
  element: Element,
  id: string,
  objectUrls: string[],
): Promise<PresentationImageElement | null> {
  const shapeProperties = first_child_by_local_name(element, "spPr");
  const transform = read_transform(shapeProperties);
  const blip = first_descendant_by_local_name(element, "blip");
  const relId = blip ? relationship_attribute(blip, "embed") || relationship_attribute(blip, "link") : undefined;
  const rel = relId ? rels[relId] : undefined;

  if (!transform || !rel || rel.target_mode === "External") {
    return null;
  }

  const mediaPath = resolve_relationship_target(slidePath, rel.target);
  const mediaFile = zip.file(mediaPath);
  if (!mediaFile) {
    return null;
  }

  const blob = await mediaFile.async("blob");
  const src = URL.createObjectURL(blob);
  objectUrls.push(src);

  return {
    ...transform,
    id,
    src,
    type: "image",
  };
}

function readPlaceholderKey(element: Element): string | undefined {
  const placeholder = first_descendant_by_local_name(element, "ph");
  if (!placeholder) {
    return undefined;
  }

  const index = placeholder.getAttribute("idx");
  if (index) {
    return `idx:${index}`;
  }

  const type = placeholder.getAttribute("type") || "body";
  return `type:${type}`;
}

function readTextAnchor(bodyProperties: Element | null): PresentationShapeElement["text_anchor"] {
  const anchor = bodyProperties?.getAttribute("anchor");
  if (anchor === "ctr") {
    return "center";
  }
  if (anchor === "b") {
    return "bottom";
  }
  return "top";
}
