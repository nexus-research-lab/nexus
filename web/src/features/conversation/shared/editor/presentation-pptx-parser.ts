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
  const object_urls: string[] = [];

  try {
    const presentation_xml = await read_zip_text(zip, "ppt/presentation.xml");
    const presentation_doc = parse_xml(presentation_xml);
    const presentation_rels = await read_relationships(zip, "ppt/presentation.xml");
    const { height, width } = read_slide_size(presentation_doc);
    const slide_paths = read_slide_paths(presentation_doc, presentation_rels);
    const resolved_slide_paths = slide_paths.length > 0 ? slide_paths : fallback_slide_paths(zip);

    if (resolved_slide_paths.length === 0) {
      throw new Error("pptx 文件中没有可预览的幻灯片");
    }

    const slides: PresentationSlide[] = [];
    for (let index = 0; index < resolved_slide_paths.length; index += 1) {
      const slide = await parse_slide(zip, resolved_slide_paths[index], index, width, height, object_urls);
      slides.push(slide);
    }

    return { object_urls, slides };
  } catch (error) {
    revoke_object_urls(object_urls);
    throw error;
  }
}

function read_slide_size(presentation_doc: Document): { height: number; width: number } {
  const slide_size = first_descendant_by_local_name(presentation_doc, "sldSz");
  const width_emu = Number(slide_size?.getAttribute("cx") || DEFAULT_SLIDE_WIDTH_EMU);
  const height_emu = Number(slide_size?.getAttribute("cy") || DEFAULT_SLIDE_HEIGHT_EMU);
  return {
    height: Math.max(emu_to_pixel(height_emu), 1),
    width: Math.max(emu_to_pixel(width_emu), 1),
  };
}

function read_slide_paths(
  presentation_doc: Document,
  presentation_rels: Record<string, PresentationRelationship>,
): string[] {
  return descendants_by_local_name(presentation_doc, "sldId")
    .map((slide_id) => {
      const rel_id = relationship_attribute(slide_id, "id");
      const rel = rel_id ? presentation_rels[rel_id] : undefined;
      return rel ? resolve_relationship_target("ppt/presentation.xml", rel.target) : null;
    })
    .filter((path): path is string => !!path);
}

function fallback_slide_paths(zip: JSZip): string[] {
  return Object.keys(zip.files)
    .filter((path) => /^ppt\/slides\/slide\d+\.xml$/i.test(path))
    .sort((left, right) => {
      const left_number = Number(left.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      const right_number = Number(right.match(/slide(\d+)\.xml$/i)?.[1] || 0);
      return left_number - right_number;
    });
}

async function parse_slide(
  zip: JSZip,
  slide_path: string,
  index: number,
  width: number,
  height: number,
  object_urls: string[],
): Promise<PresentationSlide> {
  const slide_xml = await read_zip_text(zip, slide_path);
  const slide_doc = parse_xml(slide_xml);
  const slide_rels = await read_relationships(zip, slide_path);
  const layout_path = resolve_related_part_path(slide_path, slide_rels, SLIDE_LAYOUT_RELATIONSHIP_TYPE);
  const layout_rels = layout_path ? await read_relationships(zip, layout_path) : {};
  const master_path = layout_path
    ? resolve_related_part_path(layout_path, layout_rels, SLIDE_MASTER_RELATIONSHIP_TYPE)
    : null;
  const master_part = master_path ? await parse_presentation_part(zip, master_path, object_urls) : null;
  const layout_part = layout_path
    ? await parse_presentation_part(zip, layout_path, object_urls, master_part?.placeholder_styles)
    : null;
  const inherited_placeholders = merge_placeholder_styles(
    master_part?.placeholder_styles,
    layout_part?.placeholder_styles,
  );
  const background = read_slide_background(slide_doc)
    || layout_part?.background
    || master_part?.background
    || "#ffffff";
  const shape_tree = first_descendant_by_local_name(slide_doc, "spTree");
  const slide_result = shape_tree ? await parse_shape_tree(
    zip,
    slide_path,
    slide_rels,
    shape_tree,
    object_urls,
    {
      element_index: 0,
      fallback_placeholders: inherited_placeholders,
      id_prefix: `slide-${index + 1}`,
      include_placeholder_shapes: true,
    },
  ) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };
  const elements = [
    ...(master_part?.elements ?? []),
    ...(layout_part?.elements ?? []),
    ...slide_result.elements,
  ];
  const first_text = slide_result.elements
    .flatMap((element) => element.type === "shape" ? element.paragraphs : [])
    .map((paragraph) => paragraph.text.trim())
    .find(Boolean);

  return {
    background,
    elements,
    height,
    id: `slide-${index + 1}`,
    title: first_text || `幻灯片 ${index + 1}`,
    width,
  };
}

async function parse_presentation_part(
  zip: JSZip,
  part_path: string,
  object_urls: string[],
  fallback_placeholders?: Map<string, PresentationPlaceholderStyle>,
): Promise<PresentationPart | null> {
  if (!zip.file(part_path)) {
    return null;
  }

  const part_xml = await read_zip_text(zip, part_path);
  const part_doc = parse_xml(part_xml);
  const rels = await read_relationships(zip, part_path);
  const shape_tree = first_descendant_by_local_name(part_doc, "spTree");
  const result = shape_tree ? await parse_shape_tree(zip, part_path, rels, shape_tree, object_urls, {
    element_index: 0,
    fallback_placeholders,
    id_prefix: part_path.replace(/[^a-z0-9]+/gi, "-"),
    include_placeholder_shapes: false,
  }) : { elements: [], placeholder_styles: new Map<string, PresentationPlaceholderStyle>() };

  return {
    background: read_slide_background(part_doc),
    elements: result.elements,
    placeholder_styles: result.placeholder_styles,
    rels,
  };
}

function resolve_related_part_path(
  source_path: string,
  source_rels: Record<string, PresentationRelationship>,
  relationship_type: string,
): string | null {
  const rel = Object.values(source_rels).find((relationship) => relationship.type === relationship_type);
  if (!rel || rel.target_mode === "External") {
    return null;
  }
  return resolve_relationship_target(source_path, rel.target);
}

function merge_placeholder_styles(
  base?: Map<string, PresentationPlaceholderStyle>,
  override?: Map<string, PresentationPlaceholderStyle>,
): Map<string, PresentationPlaceholderStyle> {
  return new Map([
    ...(base?.entries() ?? []),
    ...(override?.entries() ?? []),
  ]);
}

async function parse_shape_tree(
  zip: JSZip,
  slide_path: string,
  rels: Record<string, PresentationRelationship>,
  shape_tree: Element,
  object_urls: string[],
  context: PresentationShapeTreeContext,
  group_transform?: PresentationGroupTransform | null,
): Promise<PresentationShapeTreeResult> {
  const elements: PresentationElement[] = [];
  const placeholder_styles = new Map<string, PresentationPlaceholderStyle>();
  const children = Array.from(shape_tree.children);

  for (const child of children) {
    switch (child.localName) {
      case "cxnSp":
      case "sp": {
        const parsed_shape = parse_shape(child, `${context.id_prefix}-shape-${context.element_index}`, context);
        context.element_index += 1;
        if (parsed_shape.placeholder_style) {
          placeholder_styles.set(parsed_shape.placeholder_style.key, parsed_shape.placeholder_style);
        }
        if (parsed_shape.shape && (!parsed_shape.is_placeholder || context.include_placeholder_shapes)) {
          elements.push(parsed_shape.shape);
        }
        break;
      }
      case "grpSp": {
        const group_result = await parse_shape_tree(
          zip,
          slide_path,
          rels,
          child,
          object_urls,
          context,
          read_group_transform(child),
        );
        group_result.placeholder_styles.forEach((style, key) => {
          placeholder_styles.set(key, style);
        });
        elements.push(...group_result.elements);
        break;
      }
      case "pic": {
        const image = await parse_picture(
          zip,
          slide_path,
          rels,
          child,
          `${context.id_prefix}-image-${context.element_index}`,
          object_urls,
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

  if (!group_transform) {
    return { elements, placeholder_styles };
  }

  return {
    elements: elements.map((element) => apply_group_transform_to_element(element, group_transform)),
    placeholder_styles: map_group_placeholder_styles(placeholder_styles, group_transform),
  };
}

function parse_shape(
  element: Element,
  id: string,
  context: PresentationShapeTreeContext,
): {
  is_placeholder: boolean;
  placeholder_style: PresentationPlaceholderStyle | null;
  shape: PresentationShapeElement | null;
} {
  const shape_properties = first_child_by_local_name(element, "spPr");
  const placeholder_key = read_placeholder_key(element);
  const fallback_placeholder = placeholder_key ? context.fallback_placeholders?.get(placeholder_key) : undefined;
  const transform = read_transform(shape_properties) || fallback_placeholder?.transform || null;
  if (!transform) {
    return {
      is_placeholder: !!placeholder_key,
      placeholder_style: null,
      shape: null,
    };
  }

  const text_body = first_child_by_local_name(element, "txBody");
  const paragraphs = parse_text_body(text_body, transform.width);
  const text_anchor = read_text_anchor(first_child_by_local_name(text_body, "bodyPr"));
  const fill = read_fill_color(shape_properties) || fallback_placeholder?.fill;
  const stroke = read_stroke_color(shape_properties) || fallback_placeholder?.stroke;
  const stroke_width = read_stroke_width(shape_properties) || fallback_placeholder?.stroke_width || 1;
  const geometry = read_shape_geometry(shape_properties, element.localName === "cxnSp", fallback_placeholder?.geometry);
  const placeholder_style = placeholder_key ? {
    fill,
    geometry,
    key: placeholder_key,
    stroke,
    stroke_width,
    transform,
  } : null;

  if (should_skip_shape_preview({ fill, geometry, height: transform.height, paragraphs, stroke, width: transform.width })) {
    return {
      is_placeholder: !!placeholder_key,
      placeholder_style,
      shape: null,
    };
  }

  return {
    is_placeholder: !!placeholder_key,
    placeholder_style,
    shape: {
      ...transform,
      fill,
      geometry,
      id,
      paragraphs,
      stroke,
      stroke_width,
      text_anchor,
      type: "shape",
    },
  };
}

function should_skip_shape_preview({
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
    is_plain_white_fill(fill) &&
    !stroke &&
    paragraphs.length === 0 &&
    Math.min(width, height) >= MIN_BACKGROUND_LIKE_SHAPE_SIZE
  );
}

function is_plain_white_fill(fill?: string): boolean {
  const normalized_fill = fill?.toLowerCase();
  return normalized_fill === "#ffffff" || normalized_fill === "#fff";
}

async function parse_picture(
  zip: JSZip,
  slide_path: string,
  rels: Record<string, PresentationRelationship>,
  element: Element,
  id: string,
  object_urls: string[],
): Promise<PresentationImageElement | null> {
  const shape_properties = first_child_by_local_name(element, "spPr");
  const transform = read_transform(shape_properties);
  const blip = first_descendant_by_local_name(element, "blip");
  const rel_id = blip ? relationship_attribute(blip, "embed") || relationship_attribute(blip, "link") : undefined;
  const rel = rel_id ? rels[rel_id] : undefined;

  if (!transform || !rel || rel.target_mode === "External") {
    return null;
  }

  const media_path = resolve_relationship_target(slide_path, rel.target);
  const media_file = zip.file(media_path);
  if (!media_file) {
    return null;
  }

  const blob = await media_file.async("blob");
  const src = URL.createObjectURL(blob);
  object_urls.push(src);

  return {
    ...transform,
    id,
    src,
    type: "image",
  };
}

function read_placeholder_key(element: Element): string | undefined {
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

function read_text_anchor(body_properties: Element | null): PresentationShapeElement["text_anchor"] {
  const anchor = body_properties?.getAttribute("anchor");
  if (anchor === "ctr") {
    return "center";
  }
  if (anchor === "b") {
    return "bottom";
  }
  return "top";
}
