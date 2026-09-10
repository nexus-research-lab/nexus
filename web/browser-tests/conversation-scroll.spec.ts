// INPUT: Production FOLLOW hook and real browser layout across the shared browser matrix.
// OUTPUT: Live process shrink/growth, terminal and reading-position geometry regressions.
// POS: Isolated conversation scroll harness; no model or backend requests.
import { expect, test } from "@playwright/test";

test("live process collapse leaves no leading blank and continued generation follows real height", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { useFollowScroll } = await import(hookPath);
    const h = React.createElement;
    function Harness() {
      const [height, setHeight] = React.useState(450);
      const [active, setActive] = React.useState(true);
      const scroll = useFollowScroll({ messageCount: 2, contentKey: String(height), topologyKey: "round", liveLayoutActive: active, sessionKey: "test" });
      return h("div", { style: { position: "fixed", inset: 0, background: "white", zIndex: 99999 } },
        h("button", { onClick: () => setHeight(120) }, "Collapse process"),
        h("button", { onClick: () => setHeight(900) }, "Grow response"),
        h("button", { onClick: () => setActive(false) }, "Finish response"),
        h("div", { ref: scroll.scrollRef, "data-testid": "scroll", onScroll: scroll.onScroll, onWheel: scroll.onWheel, style: { height: 500, overflowY: "auto", overflowAnchor: "none" } },
          h("div", { ref: scroll.feedRef, "data-testid": "feed", style: { display: "flex", flexDirection: "column" } },
            h("div", { "data-testid": "user", "data-conversation-round-id": "user", style: { height: 50, flexShrink: 0 } }, "User prompt"),
            h("div", { "data-testid": "response", "data-conversation-round-id": "response", style: { height, flexShrink: 0 } }, "Thinking and tool process"),
            h("div", { ref: scroll.bottomAnchorRef }))));
    }
    const host = document.createElement("div");
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });
  const geometry = () => page.evaluate(() => {
    const scroll = document.querySelector<HTMLElement>('[data-testid="scroll"]')!;
    const feed = document.querySelector<HTMLElement>('[data-testid="feed"]')!;
    const user = document.querySelector<HTMLElement>('[data-testid="user"]')!;
    return {
      leading: user.getBoundingClientRect().top - feed.getBoundingClientRect().top,
      height: feed.getBoundingClientRect().height,
      top: scroll.scrollTop,
      bottomGap: scroll.scrollHeight - scroll.clientHeight - scroll.scrollTop,
    };
  });
  await expect(page.getByTestId("feed")).toBeVisible();
  await page.getByRole("button", { name: "Collapse process", exact: true }).click();
  await expect.poll(async () => (await geometry()).height).toBe(170);
  expect((await geometry()).leading).toBe(0);
  await page.getByRole("button", { name: "Grow response", exact: true }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  await page.getByTestId("scroll").hover();
  await page.mouse.wheel(0, -200);
  await expect.poll(async () => (await geometry()).top).toBeLessThan(450);
  const readingTop = (await geometry()).top;
  await page.getByRole("button", { name: "Finish response", exact: true }).click();
  expect((await geometry()).top).toBe(readingTop);
  await page.getByRole("button", { name: "Collapse process", exact: true }).click();
  await expect.poll(async () => (await geometry()).height).toBe(170);
  expect((await geometry()).leading).toBe(0);
});

for (const virtual of [false, true]) {
  test(`${virtual ? "virtual" : "static"} live resizing keeps the visible tail stable between alternating size commits`, async ({ page }) => {
    await page.goto("/ui-gallery.html");
    const samples = await page.evaluate(async (virtual) => {
      const modules = {
        react: "/node_modules/.vite-browser-test/deps/react.js",
        dom: "/node_modules/.vite-browser-test/deps/react-dom_client.js",
        virtual: "/node_modules/.vite-browser-test/deps/@tanstack_react-virtual.js",
        hook: "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts",
        policy: "/src/features/conversation/shared/feed/use-conversation-virtual-scroll-policy.ts",
        canvas: "/src/features/conversation/shared/feed/conversation-virtual-canvas.tsx",
      };
      const { default: React } = await import(modules.react);
      const { default: { createRoot } } = await import(modules.dom);
      const { useVirtualizer } = await import(modules.virtual);
      const { useFollowScroll } = await import(modules.hook);
      const { shouldAdjustConversationVirtualScrollPosition } = await import(modules.policy);
      const { ConversationVirtualCanvas } = await import(modules.canvas);
      const h = React.createElement;
      let resize: (height: number) => void = () => {};
      let pause: () => void = () => {};
      // rAF runs before ResizeObserver; sample in the next task after the rendering
      // cycle so unfinished virtual measurements are not mistaken for painted jumps.
      const frame = () => new Promise<void>((resolve) => requestAnimationFrame(() => setTimeout(resolve, 0)));
      function Harness() {
        const [height, setHeight] = React.useState(300);
        resize = setHeight;
        const scroll = useFollowScroll({ messageCount: 30, contentKey: String(height), topologyKey: "fixed-rounds", liveLayoutActive: true, sessionKey: "stress" });
        pause = scroll.pauseFollowLatest;
        const v = useVirtualizer({ count: 30, enabled: virtual, getScrollElement: () => scroll.scrollRef.current,
          estimateSize: (index: number) => index === 28 ? 300 : 100,
          measureElement: (element: HTMLElement) => element.getBoundingClientRect().height, overscan: 5 });
        v.shouldAdjustScrollPositionOnItemSizeChange = (item: { end: number }, delta: number, instance: { scrollOffset: number | null }) => shouldAdjustConversationVirtualScrollPosition(item, delta, instance, {
          bottomScrollActive: scroll.isBottomScrollActive(), followingLatest: scroll.isFollowingLatest(), userScrollActive: scroll.isUserScrollActive(),
        });
        const row = (index: number) => h("div", { key: index, ref: virtual ? v.measureElement : undefined,
          "data-index": index, "data-conversation-round-id": String(index), "data-stress-tail": index === 29 ? "true" : undefined,
          style: { height: index === 28 ? height : 100, flexShrink: 0 } }, `Round ${index}`);
        return h("div", { ref: scroll.scrollRef, "data-stress-scroll": "true", onScroll: scroll.onScroll,
          style: { position: "fixed", inset: 0, height: 500, overflowY: "auto", overflowAnchor: "none", scrollbarGutter: "stable", background: "white", zIndex: 99999 } },
        h("div", { ref: scroll.feedRef, "data-conversation-virtual-feed": virtual ? "true" : undefined,
          style: virtual ? { position: "relative", height: v.getTotalSize() } : { display: "flex", flexDirection: "column" } },
        virtual ? h(ConversationVirtualCanvas, { totalSize: v.getTotalSize(), offset: v.getVirtualItems()[0]?.start ?? 0 }, v.getVirtualItems().map((item: { index: number }) => row(item.index)))
          : Array.from({ length: 30 }, (_, index) => row(index)),
        h("div", { ref: scroll.bottomAnchorRef, style: virtual ? { position: "absolute", bottom: 0 } : undefined })));
      }
      const host = document.createElement("div");
      document.body.append(host);
      createRoot(host).render(h(Harness));
      for (let i = 0; i < 20; i++) await frame();
      const viewport = host.querySelector<HTMLElement>("[data-stress-scroll]")!;
      const samples: { phase: string; gap: number }[] = [];
      for (const height of [150, 370, 180, 410, 210, 450]) {
        resize(height);
        for (let i = 0; i < 8; i++) {
          await frame();
          const tail = host.querySelector<HTMLElement>("[data-stress-tail]")!;
          samples.push({ phase: `follow-${height}-${i}`, gap: viewport.getBoundingClientRect().bottom - tail.getBoundingClientRect().bottom });
        }
      }
      viewport.scrollTop -= 200;
      pause();
      for (let i = 0; i < 3; i++) await frame();
      const readingTop = viewport.scrollTop;
      for (const height of [500, 460, 550, 480]) {
        resize(height);
        for (let i = 0; i < 8; i++) {
          await frame();
          samples.push({ phase: `reading-${height}-${i}`, gap: viewport.scrollTop - readingTop });
        }
      }
      return samples;
    }, virtual);
    expect(samples.filter((sample) => Math.abs(sample.gap) > 1)).toEqual([]);
  });
}
