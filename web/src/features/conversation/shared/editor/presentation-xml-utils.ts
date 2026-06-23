import type JSZip from "jszip";

import {
  EMU_PER_PIXEL,
  RELATIONSHIP_NAMESPACE,
  type PresentationRelationship,
} from "./presentation-preview-model";

export async function read_relationships(
  zip: JSZip,
  part_path: string,
): Promise<Record<string, PresentationRelationship>> {
  const rels_path = relationship_part_path(part_path);
  const rels_file = zip.file(rels_path);
  if (!rels_file) {
    return {};
  }

  const rels_doc = parse_xml(await rels_file.async("text"));
  const relationships: Record<string, PresentationRelationship> = {};

  descendants_by_local_name(rels_doc, "Relationship").forEach((relationship) => {
    const id = relationship.getAttribute("Id");
    const target = relationship.getAttribute("Target");
    if (!id || !target) {
      return;
    }

    relationships[id] = {
      target,
      target_mode: relationship.getAttribute("TargetMode") || undefined,
      type: relationship.getAttribute("Type") || undefined,
    };
  });

  return relationships;
}

export async function read_zip_text(zip: JSZip, file_path: string): Promise<string> {
  const file = zip.file(file_path);
  if (!file) {
    throw new Error(`pptx 缺少 ${file_path}`);
  }
  return file.async("text");
}

export function parse_xml(xml: string): Document {
  const doc = new DOMParser().parseFromString(xml, "application/xml");
  const parse_error = first_descendant_by_local_name(doc, "parsererror");
  if (parse_error) {
    throw new Error("pptx XML 解析失败");
  }
  return doc;
}

export function relationship_attribute(element: Element, local_name: string): string | undefined {
  return Array.from(element.attributes)
    .find((attribute) => attribute.localName === local_name && attribute.namespaceURI === RELATIONSHIP_NAMESPACE)
    ?.value;
}

function relationship_part_path(part_path: string): string {
  const normalized_path = normalize_zip_path(part_path);
  const parts = normalized_path.split("/");
  const file_name = parts.pop();
  return normalize_zip_path(`${parts.join("/")}/_rels/${file_name}.rels`);
}

export function resolve_relationship_target(source_path: string, target: string): string {
  if (target.startsWith("/")) {
    return normalize_zip_path(target);
  }

  const source_parts = normalize_zip_path(source_path).split("/");
  source_parts.pop();
  return normalize_zip_path(`${source_parts.join("/")}/${target}`);
}

function normalize_zip_path(file_path: string): string {
  const segments: string[] = [];
  file_path.replace(/\\/g, "/").split("/").forEach((segment) => {
    if (!segment || segment === ".") {
      return;
    }
    if (segment === "..") {
      segments.pop();
      return;
    }
    segments.push(segment);
  });
  return segments.join("/");
}

export function emu_to_pixel(value: number): number {
  return value / EMU_PER_PIXEL;
}

export function children_by_local_name(element: Element | null, local_name: string): Element[] {
  if (!element) {
    return [];
  }
  return Array.from(element.children).filter((child) => child.localName === local_name);
}

export function first_child_by_local_name(element: Element | null, local_name: string): Element | null {
  return children_by_local_name(element, local_name)[0] || null;
}

export function descendants_by_local_name(root: Document | Element, local_name: string): Element[] {
  return Array.from(root.getElementsByTagName("*")).filter((element) => element.localName === local_name);
}

export function first_descendant_by_local_name(root: Document | Element | null, local_name: string): Element | null {
  if (!root) {
    return null;
  }
  return descendants_by_local_name(root, local_name)[0] || null;
}

export function revoke_object_urls(urls: string[]) {
  urls.forEach((url) => URL.revokeObjectURL(url));
}
