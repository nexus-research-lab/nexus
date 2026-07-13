const CSS_UNIT_TO_PIXELS: Readonly<Record<string, number>> = {
  cm: 96 / 2.54,
  in: 96,
  mm: 96 / 25.4,
  pt: 96 / 72,
  px: 1,
};

export function calculateDocumentPreviewScale(
  viewport: HTMLElement,
  container: HTMLElement,
): number {
  const pageWidth = getDocumentPageWidth(container);
  if (pageWidth <= 0) {
    return 1;
  }

  const availableWidth = Math.max(viewport.clientWidth - 40, 1);
  const scale = Math.max(Math.min(availableWidth / pageWidth, 1), 0.1);
  return Math.round(scale * 1000) / 1000;
}

export function normalizeDocumentMedia(container: HTMLElement): void {
  const mediaElements = container.querySelectorAll<HTMLElement>(
    "section.nexus-docx-preview img, section.nexus-docx-preview svg",
  );
  mediaElements.forEach((media) => normalizeMediaElement(media));
}

function getDocumentPageWidth(container: HTMLElement): number {
  const pages = container.querySelectorAll<HTMLElement>(
    "section.nexus-docx-preview",
  );
  return Array.from(pages).reduce((maxWidth, page) => {
    const layoutWidth = page.offsetWidth
      || page.clientWidth
      || page.getBoundingClientRect().width;
    return Math.max(
      maxWidth,
      parseCssLengthToPixels(page.style.width),
      layoutWidth,
      page.scrollWidth,
    );
  }, 0);
}

function normalizeMediaElement(media: HTMLElement): void {
  const page = media.closest<HTMLElement>("section.nexus-docx-preview");
  if (!page) {
    return;
  }

  const mediaWidth = getUnscaledElementWidth(media);
  const widthLimit = getMediaWidthLimit(media, page, mediaWidth);
  if (mediaWidth <= 0 || widthLimit <= 0 || mediaWidth <= widthLimit + 1) {
    return;
  }

  const nextWidth = `${Math.floor(widthLimit)}px`;
  Object.assign(media.style, {
    height: "auto",
    maxWidth: "100%",
    objectFit: "contain",
    width: nextWidth,
  });
  normalizeMediaParent(media, page, nextWidth);
}

function normalizeMediaParent(
  media: HTMLElement,
  page: HTMLElement,
  width: string,
): void {
  const parent = media.parentElement;
  if (!parent || parent === page) {
    return;
  }

  Object.assign(parent.style, {
    height: "auto",
    maxWidth: "100%",
    overflow: "hidden",
    width,
  });
}

function getMediaWidthLimit(
  media: HTMLElement,
  page: HTMLElement,
  mediaWidth: number,
): number {
  const pageStyle = window.getComputedStyle(page);
  const pageContentWidth = page.clientWidth
    - parseCssLengthToPixels(pageStyle.paddingLeft)
    - parseCssLengthToPixels(pageStyle.paddingRight);
  const candidates = pageContentWidth > 0 ? [pageContentWidth] : [];
  let current = media.parentElement;

  while (current && current !== page) {
    const style = window.getComputedStyle(current);
    const constrainsMedia = style.display !== "inline"
      && current.clientWidth > 0
      && current.clientWidth < mediaWidth;
    if (constrainsMedia) {
      candidates.push(current.clientWidth);
    }
    current = current.parentElement;
  }

  return candidates.length > 0 ? Math.max(Math.min(...candidates), 120) : 0;
}

function getUnscaledElementWidth(element: HTMLElement): number {
  return parseCssLengthToPixels(element.style.width)
    || Number(element.getAttribute("width") || 0)
    || element.scrollWidth
    || element.offsetWidth
    || element.getBoundingClientRect().width;
}

function parseCssLengthToPixels(value: string): number {
  const match = value.trim().match(/^(-?\d+(?:\.\d+)?)(px|pt|in|cm|mm)?$/i);
  if (!match) {
    return 0;
  }

  const amount = Number(match[1]);
  const unit = match[2]?.toLowerCase() || "px";
  return amount * (CSS_UNIT_TO_PIXELS[unit] ?? 1);
}
