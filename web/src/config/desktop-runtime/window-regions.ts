/**
 * INPUT: 页面声明的桌面 Header 与其中可交互元素。
 * OUTPUT: Windows WPF 宿主可消费的拖动区和硬排除区矩形。
 * POS: 只投影 DOM 几何，不创建视觉层或执行窗口命令。
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
const HARD_NO_DRAG_SELECTOR = [
  "audio[controls]",
  "iframe",
  "input",
  "select",
  "textarea",
  "video[controls]",
  "[contenteditable]:not([contenteditable='false'])",
  "[data-desktop-window-no-drag]",
].join(",");

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
    const noDragElements = uniqueElements(
      dragElements.flatMap((region) =>
        Array.from(region.querySelectorAll(HARD_NO_DRAG_SELECTOR)),
      ),
    );
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

function uniqueElements(elements: Element[]): Element[] {
  return Array.from(new Set(elements));
}

function sameElements(left: Element[], right: Element[]): boolean {
  return (
    left.length === right.length
    && left.every((element, index) => element === right[index])
  );
}

function normalized(value: number): number {
  return Math.round(value * 4) / 4;
}
