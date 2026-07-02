import type JSZip from "jszip";

import {
  EMU_PER_PIXEL,
  RELATIONSHIP_NAMESPACE,
  type PresentationRelationship,
} from "./presentation-preview-model";

export async function read_relationships(
  zip: JSZip,
  partPath: string,
): Promise<Record<string, PresentationRelationship>> {
  const relsPath = relationshipPartPath(partPath);
  const relsFile = zip.file(relsPath);
  if (!relsFile) {
    return {};
  }

  const relsDoc = parse_xml(await relsFile.async("text"));
  const relationships: Record<string, PresentationRelationship> = {};

  descendants_by_local_name(relsDoc, "Relationship").forEach((relationship) => {
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

export async function read_zip_text(zip: JSZip, filePath: string): Promise<string> {
  const file = zip.file(filePath);
  if (!file) {
    throw new Error(`pptx 缺少 ${filePath}`);
  }
  return file.async("text");
}

export function parse_xml(xml: string): Document {
  const doc = new DOMParser().parseFromString(xml, "application/xml");
  const parseError = first_descendant_by_local_name(doc, "parsererror");
  if (parseError) {
    throw new Error("pptx XML 解析失败");
  }
  return doc;
}

export function relationship_attribute(element: Element, localName: string): string | undefined {
  return Array.from(element.attributes)
    .find((attribute) => attribute.localName === localName && attribute.namespaceURI === RELATIONSHIP_NAMESPACE)
    ?.value;
}

function relationshipPartPath(partPath: string): string {
  const normalizedPath = normalizeZipPath(partPath);
  const parts = normalizedPath.split("/");
  const fileName = parts.pop();
  return normalizeZipPath(`${parts.join("/")}/_rels/${fileName}.rels`);
}

export function resolve_relationship_target(sourcePath: string, target: string): string {
  if (target.startsWith("/")) {
    return normalizeZipPath(target);
  }

  const sourceParts = normalizeZipPath(sourcePath).split("/");
  sourceParts.pop();
  return normalizeZipPath(`${sourceParts.join("/")}/${target}`);
}

function normalizeZipPath(filePath: string): string {
  const segments: string[] = [];
  filePath.replace(/\\/g, "/").split("/").forEach((segment) => {
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

export function children_by_local_name(element: Element | null, localName: string): Element[] {
  if (!element) {
    return [];
  }
  return Array.from(element.children).filter((child) => child.localName === localName);
}

export function first_child_by_local_name(element: Element | null, localName: string): Element | null {
  return children_by_local_name(element, localName)[0] || null;
}

export function descendants_by_local_name(root: Document | Element, localName: string): Element[] {
  return Array.from(root.getElementsByTagName("*")).filter((element) => element.localName === localName);
}

export function first_descendant_by_local_name(root: Document | Element | null, localName: string): Element | null {
  if (!root) {
    return null;
  }
  return descendants_by_local_name(root, localName)[0] || null;
}

export function revoke_object_urls(urls: string[]) {
  urls.forEach((url) => URL.revokeObjectURL(url));
}
