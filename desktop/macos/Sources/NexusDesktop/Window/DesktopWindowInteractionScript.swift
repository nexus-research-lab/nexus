/**
 * INPUT: 页面中显式声明的桌面拖动区域。
 * OUTPUT: 视口坐标系下的拖动矩形与编辑控件排除矩形。
 * POS: Web DOM 到 macOS 窗口事件模型的只读投影。
 */
enum DesktopWindowInteractionScript {
  static func make() -> String {
    """
    (() => {
      if (window.__NEXUS_DESKTOP_WINDOW_INTERACTION__) {
        return;
      }

      const dragRegionSelector = "[data-desktop-window-drag-region]";
      const hardNoDragSelector = [
        "input",
        "select",
        "textarea",
        "[contenteditable]:not([contenteditable='false'])",
        "audio[controls]",
        "video[controls]",
        "iframe",
        "[data-desktop-window-no-drag]",
      ].join(",");

      let animationFrame = 0;
      let lastSignature = "";
      let observedElements = [];

      function normalized(value) {
        return Math.round(value * 4) / 4;
      }

      function visibleRect(element) {
        const style = window.getComputedStyle(element);
        if (
          style.display === "none"
          || style.visibility === "hidden"
          || style.pointerEvents === "none"
        ) {
          return null;
        }
        const rect = element.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0) {
          return null;
        }
        return {
          x: normalized(rect.left),
          y: normalized(rect.top),
          width: normalized(rect.width),
          height: normalized(rect.height),
        };
      }

      function sameElements(left, right) {
        return (
          left.length === right.length
          && left.every((element, index) => element === right[index])
        );
      }

      const resizeObserver = new ResizeObserver(() => schedulePublish());

      function synchronizeObservedElements(elements) {
        if (sameElements(observedElements, elements)) {
          return;
        }
        resizeObserver.disconnect();
        observedElements.length = 0;
        elements.forEach((element) => {
          observedElements.push(element);
          resizeObserver.observe(element);
        });
      }

      function publishRegions() {
        animationFrame = 0;
        const dragElements = Array.from(
          document.querySelectorAll(dragRegionSelector),
        );
        const noDragElements = Array.from(new Set(
          dragElements.flatMap((region) => (
            Array.from(region.querySelectorAll(hardNoDragSelector))
          )),
        ));
        synchronizeObservedElements([...dragElements, ...noDragElements]);

        const dragRegions = dragElements.map(visibleRect).filter(Boolean);
        const noDragRegions = noDragElements.map(visibleRect).filter(Boolean);
        const signature = JSON.stringify([dragRegions, noDragRegions]);
        if (signature === lastSignature) {
          return;
        }
        lastSignature = signature;
        window.webkit?.messageHandlers?.nexusDesktopWindow?.postMessage({
          schema_version: 1,
          kind: "regions_changed",
          drag_regions: dragRegions,
          no_drag_regions: noDragRegions,
        });
      }

      function schedulePublish() {
        if (animationFrame !== 0) {
          return;
        }
        animationFrame = window.requestAnimationFrame(publishRegions);
      }

      function nodeContainsDragRegion(node) {
        return (
          node instanceof Element
          && (
            node.matches(dragRegionSelector)
            || Boolean(node.querySelector(dragRegionSelector))
          )
        );
      }

      function mutationTouchesDragRegion(mutation) {
        if (
          mutation.type === "attributes"
          && mutation.attributeName === "data-desktop-window-drag-region"
        ) {
          return true;
        }
        const target = mutation.target instanceof Element
          ? mutation.target
          : mutation.target.parentElement;
        if (target?.closest(dragRegionSelector)) {
          return true;
        }
        return (
          Array.from(mutation.addedNodes).some(nodeContainsDragRegion)
          || Array.from(mutation.removedNodes).some(nodeContainsDragRegion)
        );
      }

      function start() {
        const mutationObserver = new MutationObserver((mutations) => {
          if (mutations.some(mutationTouchesDragRegion)) {
            schedulePublish();
          }
        });
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
        document.fonts?.ready.then(schedulePublish);
        schedulePublish();
      }

      if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", start, { once: true });
      } else {
        start();
      }

      window.__NEXUS_DESKTOP_WINDOW_INTERACTION__ = true;
    })();
    """
  }
}
