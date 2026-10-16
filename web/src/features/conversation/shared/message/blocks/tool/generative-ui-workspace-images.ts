/**
 * INPUT: show_widget HTML and the exact Agent workspace identity.
 * OUTPUT: HTML with authenticated workspace image references replaced by Blob URLs.
 * POS: Host-side resource bridge; the generated iframe never receives cookies or same-origin access.
 */

const WORKSPACE_IMAGE_SCHEME = "nexus:";

function workspaceImagePath(value: string): string | null {
  try {
    const url = new URL(value);
    if (url.protocol !== WORKSPACE_IMAGE_SCHEME || url.hostname !== "workspace") return null;
    const path = decodeURIComponent(url.pathname.replace(/^\/+/, "")).trim();
    return path && !url.search && !url.hash ? path : null;
  } catch {
    return null;
  }
}

export async function materializeWorkspaceImages(
  html: string,
  agentId: string | null | undefined,
  objectUrls: Map<string, string>,
  getPreviewUrl: (agentId: string, path: string) => string,
): Promise<string> {
  if (!agentId || typeof DOMParser === "undefined") return html;
  const document = new DOMParser().parseFromString(html, "text/html");
  const elements = Array.from(document.querySelectorAll("img[src], source[src], video[poster], audio[poster]"));
  const references = new Set<string>();
  for (const element of elements) {
    for (const attribute of ["src", "poster"]) {
      const path = workspaceImagePath(element.getAttribute(attribute) ?? "");
      if (path) references.add(path);
    }
  }
  await Promise.all(Array.from(references).map(async (path) => {
    if (objectUrls.has(path)) return;
    try {
      const response = await fetch(getPreviewUrl(agentId, path), { credentials: "include" });
      if (response.ok) objectUrls.set(path, URL.createObjectURL(await response.blob()));
    } catch {
      // Keep the original reference so the iframe can report a broken image.
    }
  }));
  for (const element of elements) {
    for (const attribute of ["src", "poster"]) {
      const path = workspaceImagePath(element.getAttribute(attribute) ?? "");
      const objectUrl = path ? objectUrls.get(path) : undefined;
      if (objectUrl) element.setAttribute(attribute, objectUrl);
    }
  }
  return document.body.innerHTML;
}
