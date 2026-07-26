/**
 * INPUT: Header 的 app-region 计算样式。
 * OUTPUT: Windows WPF 宿主可消费的拖动与交互矩形。
 * POS: WebView2 WPF wrapper 的非客户区几何适配，不拥有手势。
 */

interface DesktopWindowRegion {
  height: number;
  width: number;
  x: number;
  y: number;
}

interface DesktopWindowRegionHandler {
  postMessage: (message: Record<string, unknown>) => void;
}

interface DesktopWindowBridge {
  webkit?: {
    messageHandlers?: {
      nexusDesktopWindow?: DesktopWindowRegionHandler;
    };
  };
}

const DRAG_REGION_SELECTOR = "[data-desktop-window-drag-region]";

export function startWindowsWindowRegionSync(): void {
  const bridge = window as Window & DesktopWindowBridge;
  const handler = bridge.webkit?.messageHandlers?.nexusDesktopWindow;
  if (!handler || document.documentElement.dataset.windowRegionSync === "true") {
    return;
  }

  document.documentElement.dataset.windowRegionSync = "true";
  installWindowRegionSync(handler);
}

function installWindowRegionSync(handler: DesktopWindowRegionHandler): void {
  let animationFrame = 0;
  let lastSignature = "";
  let observedElements: Element[] = [];

  const resizeObserver = new ResizeObserver(schedulePublish);
  const mutationObserver = new MutationObserver((mutations) => {
    if (mutations.some(mutationTouchesDragRegion)) schedulePublish();
  });

  function publishRegions(): void {
    animationFrame = 0;
    const dragElements = Array.from(
      document.querySelectorAll(DRAG_REGION_SELECTOR),
    );
    const noDragElements = Array.from(new Set(
      dragElements.flatMap((region) =>
        Array.from(region.querySelectorAll("*")).filter(isNoDragElement)
      ),
    ));
    const nextObservedElements = [...dragElements, ...noDragElements];
    if (!sameElements(observedElements, nextObservedElements)) {
      resizeObserver.disconnect();
      observedElements = nextObservedElements;
      observedElements.forEach((element) => resizeObserver.observe(element));
    }

    const dragRegions = visibleRegions(dragElements);
    const noDragRegions = visibleRegions(noDragElements);
    const signature = JSON.stringify([dragRegions, noDragRegions]);
    if (signature === lastSignature) return;

    lastSignature = signature;
    handler.postMessage({
      schema_version: 1,
      kind: "regions_changed",
      drag_regions: dragRegions,
      no_drag_regions: noDragRegions,
    });
  }

  function schedulePublish(): void {
    if (animationFrame !== 0) return;
    animationFrame = window.requestAnimationFrame(publishRegions);
  }

  mutationObserver.observe(document, {
    attributeFilter: [
      "class",
      "contenteditable",
      "controls",
      "data-desktop-window-drag-region",
      "data-desktop-window-no-drag",
      "disabled",
      "draggable",
      "hidden",
      "href",
      "role",
      "style",
      "tabindex",
    ],
    attributes: true,
    childList: true,
    subtree: true,
  });
  window.addEventListener("resize", schedulePublish);
  window.addEventListener("scroll", schedulePublish, true);
  void document.fonts?.ready.then(schedulePublish);
  schedulePublish();
}

function isNoDragElement(element: Element): boolean {
  const style = window.getComputedStyle(element);
  const appRegion = style.getPropertyValue("app-region")
    || style.getPropertyValue("-webkit-app-region");
  return appRegion.trim() === "no-drag";
}

function visibleRegions(elements: Element[]): DesktopWindowRegion[] {
  return elements
    .map(visibleRegion)
    .filter((region): region is DesktopWindowRegion => region !== null);
}

function visibleRegion(element: Element): DesktopWindowRegion | null {
  const style = window.getComputedStyle(element);
  if (
    style.display === "none"
    || style.visibility === "hidden"
    || style.pointerEvents === "none"
  ) {
    return null;
  }

  const rect = element.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return null;
  return {
    height: normalized(rect.height),
    width: normalized(rect.width),
    x: normalized(rect.left),
    y: normalized(rect.top),
  };
}

function mutationTouchesDragRegion(mutation: MutationRecord): boolean {
  if (
    mutation.type === "attributes"
    && mutation.attributeName === "data-desktop-window-drag-region"
  ) {
    return true;
  }

  const target = mutation.target instanceof Element
    ? mutation.target
    : mutation.target.parentElement;
  if (target?.closest(DRAG_REGION_SELECTOR)) return true;
  return [...mutation.addedNodes, ...mutation.removedNodes]
    .some(nodeContainsDragRegion);
}

function nodeContainsDragRegion(node: Node): boolean {
  return node instanceof Element && (
    node.matches(DRAG_REGION_SELECTOR)
    || Boolean(node.querySelector(DRAG_REGION_SELECTOR))
  );
}

function sameElements(left: Element[], right: Element[]): boolean {
  return left.length === right.length
    && left.every((element, index) => element === right[index]);
}

function normalized(value: number): number {
  return Math.round(value * 4) / 4;
}
