// INPUT: React root and explicit data-bootstrap-pending route/auth placeholders.
// OUTPUT: One readiness notification after committed content paints, with cancellation.
// POS: Bootstrap readiness lifecycle; loading placeholders cannot declare the app ready.
export function observeRootReadiness(
  container: HTMLElement,
  notify: (source: string) => void,
): () => void {
  let finished = false;
  let frame = 0;
  let timer = 0;
  const hasContent = () => container.childElementCount > 0
    && !container.querySelector("[data-bootstrap-pending]");
  const cleanup = () => {
    finished = true;
    observer.disconnect();
    cancelAnimationFrame(frame);
    window.clearTimeout(timer);
  };
  const complete = (source: string) => {
    if (finished || !hasContent()) return;
    cleanup();
    notify(source);
  };
  const schedule = () => {
    if (finished || frame || timer || !hasContent()) return;
    frame = requestAnimationFrame(() => {
      frame = requestAnimationFrame(() => {
        frame = 0;
        complete("afterPaint");
      });
    });
    timer = window.setTimeout(() => {
      timer = 0;
      complete("timerFallback");
    }, 250);
  };
  const observer = new MutationObserver(schedule);
  observer.observe(container, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["data-bootstrap-pending"],
  });
  schedule();
  return cleanup;
}
