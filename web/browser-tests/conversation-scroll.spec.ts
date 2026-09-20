// INPUT: Production FOLLOW hook and real browser layout across the shared browser matrix.
// OUTPUT: Live process shrink/growth, terminal/reading geometry and bottom-control visibility regressions.
// POS: Isolated conversation scroll harness; no model or backend requests.
import { expect, test } from "@playwright/test";

test("generation scroll control stays out of the bottom layout until the reader leaves bottom", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const layoutPath = "/src/features/conversation/shared/conversation-panel-layout.tsx";
    const modelPath = "/src/features/conversation/shared/conversation-panel-model.ts";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { I18nProvider } = await import(i18nPath);
    const {
      ConversationPanelBottomArea,
      ConversationPanelLayout,
      ConversationPanelViewport,
      ConversationPanelViewportArea,
    } = await import(layoutPath);
    const { buildConversationPanelFrameModel } = await import(modelPath);
    const h = React.createElement;
    const healthyReliability = {
      failure: null,
      provider_retry: null,
      transport_phase: "healthy",
    };
    function Harness() {
      const [isLoading, setIsLoading] = React.useState(true);
      const [readingEarlier, setReadingEarlier] = React.useState(false);
      const scrollRef = React.useRef(null);
      const frame = buildConversationPanelFrameModel({
        conversation: {
          is_history_loading: false,
          is_loading: isLoading,
          is_session_loading: false,
          load_round_window: () => undefined,
          load_session: () => undefined,
          reliability: healthyReliability,
        },
        history: { handleScroll: () => undefined },
        roundIndexResource: {
          access: false,
          error: null,
          isLoading: false,
          isStale: false,
          retry: () => undefined,
        },
        roundScrollRef: { current: null },
        scroll: {
          isFollowingLatest: () => !readingEarlier,
          onPointerDown: () => undefined,
          onScroll: () => undefined,
          onTouchEnd: () => undefined,
          onTouchMove: () => undefined,
          onTouchStart: () => undefined,
          onWheel: () => undefined,
          reconcileFollowLatest: () => undefined,
          scrollRef,
          scrollToBottom: () => undefined,
          showScrollToBottom: readingEarlier,
        },
        sessionKey: "scroll-control-test",
        timeline: {},
      }, { isMobileLayout: false, providerWarningVisible: false }, {});
      return h(I18nProvider, null,
        h(ConversationPanelLayout, null,
          h(ConversationPanelViewportArea, null,
            h(ConversationPanelViewport, {
              floatingDockOccupied: frame.scrollToLatest.visible,
              isMobileLayout: false,
              viewport: frame.viewport,
            }, h("div", { style: { height: 1000 } }, "feed")),
          ),
          h(ConversationPanelBottomArea, {
            children: h("div", { style: { height: 80 } }),
            goal: null,
            isMobileLayout: false,
            isReconciling: false,
            onReconcile: () => undefined,
            providerWarningVisible: false,
            reliability: healthyReliability,
            scrollToLatest: frame.scrollToLatest,
          }),
          h("button", {
            onClick: () => setReadingEarlier((value: boolean) => !value),
          }, "Read earlier"),
          h("button", {
            onClick: () => setIsLoading((value: boolean) => !value),
          }, "Toggle generation"),
        ),
      );
    }
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0;z-index:99999;background:white";
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });

  await expect(page.locator("[data-scroll-to-latest]")).toHaveCount(0);
  await expect(page.locator("[data-conversation-dock-clearance]")).toHaveCount(0);

  await page.getByRole("button", { name: "Read earlier" }).click();
  await expect(page.locator("[data-scroll-to-latest]")).toHaveCount(1);
  await expect(page.locator("[data-scroll-to-latest]")).toHaveAttribute("data-generating", "true");
  await expect(page.locator("[data-conversation-dock-clearance]")).toHaveCount(1);

  await page.getByRole("button", { name: "Toggle generation" }).click();
  await expect(page.locator("[data-scroll-to-latest]")).not.toHaveAttribute("data-generating", "true");

  await page.getByRole("button", { name: "Read earlier" }).click();
  await expect(page.locator("[data-scroll-to-latest]")).toHaveCount(0);
  await expect(page.locator("[data-conversation-dock-clearance]")).toHaveCount(0);
});

test("activity Dock clearance keeps FOLLOW pinned at the real bottom", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
    const viewportPath = "/src/features/conversation/shared/conversation-panel-layout.tsx";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { I18nProvider } = await import(i18nPath);
    const { useFollowScroll } = await import(hookPath);
    const { ConversationPanelViewport } = await import(viewportPath);
    const h = React.createElement;
    function Harness() {
      const [occupied, setOccupied] = React.useState(false);
      const scroll = useFollowScroll({
        contentKey: "fixed",
        messageCount: 1,
        sessionKey: "dock-test",
        topologyKey: "fixed",
      });
      return h(I18nProvider, null,
        h("div", { style: { position: "fixed", inset: 0, display: "flex", flexDirection: "column", background: "white" } },
          h("button", { "data-dock-toggle": true, onClick: () => setOccupied((value: boolean) => !value) }, "Toggle activity"),
          h("div", { style: { minHeight: 0, flex: 1, display: "flex" } },
            h(ConversationPanelViewport, {
              floatingDockOccupied: occupied,
              isMobileLayout: false,
              viewport: {
                isFollowingLatest: scroll.isFollowingLatest,
                isHistoryLoading: false,
                onPointerDown: scroll.onPointerDown,
                onScroll: scroll.onScroll,
                onTouchEnd: scroll.onTouchEnd,
                onTouchMove: scroll.onTouchMove,
                onTouchStart: scroll.onTouchStart,
                onWheel: scroll.onWheel,
                reconcileFollowLatest: scroll.reconcileFollowLatest,
                scrollRef: scroll.scrollRef,
              },
            },
              h("div", { ref: scroll.feedRef, style: { height: 1000, flexShrink: 0 } }, "feed"),
            ),
          ),
        ),
      );
    }
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0;z-index:99999;background:white";
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });
  const viewport = page.locator(".overflow-y-auto").last();
  const geometry = () => viewport.evaluate((element) => ({
    bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
    scrollHeight: element.scrollHeight,
  }));
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  const before = await geometry();
  await page.getByRole("button", { name: "Toggle activity" }).click();
  await expect.poll(async () => (await geometry()).scrollHeight).toBe(before.scrollHeight + 56);
  expect((await geometry()).bottomGap).toBe(0);
  await page.getByRole("button", { name: "Toggle activity" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);

  await viewport.hover();
  await page.mouse.wheel(0, -100);
  await expect.poll(async () => (await geometry()).bottomGap).toBe(100);
  const readingTop = await viewport.evaluate((element) => element.scrollTop);
  await page.getByRole("button", { name: "Toggle activity" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(156);
  expect(await viewport.evaluate((element) => element.scrollTop)).toBe(readingTop);
  await page.getByRole("button", { name: "Toggle activity" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(100);
  expect(await viewport.evaluate((element) => element.scrollTop)).toBe(readingTop);
});

test("wrapped activity Dock clearance tracks its measured height without moving READING", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
    const layoutPath = "/src/features/conversation/shared/conversation-panel-layout.tsx";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { I18nProvider } = await import(i18nPath);
    const {
      ConversationPanelBottomArea,
      ConversationPanelLayout,
      ConversationPanelViewport,
      ConversationPanelViewportArea,
    } = await import(layoutPath);
    const { useFollowScroll } = await import(hookPath);
    const h = React.createElement;
    function Harness() {
      const [activityHeight, setActivityHeight] = React.useState(46);
      const scroll = useFollowScroll({
        contentKey: String(activityHeight),
        messageCount: 1,
        sessionKey: "wrapped-dock-test",
        topologyKey: String(activityHeight),
      });
      return h(I18nProvider, null,
        h(ConversationPanelLayout, null,
          h(ConversationPanelViewportArea, null,
            h(ConversationPanelViewport, {
              floatingDockOccupied: true,
              isMobileLayout: true,
              viewport: {
                isFollowingLatest: scroll.isFollowingLatest,
                isHistoryLoading: false,
                onPointerDown: scroll.onPointerDown,
                onScroll: scroll.onScroll,
                onTouchEnd: scroll.onTouchEnd,
                onTouchMove: scroll.onTouchMove,
                onTouchStart: scroll.onTouchStart,
                onWheel: scroll.onWheel,
                reconcileFollowLatest: scroll.reconcileFollowLatest,
                scrollRef: scroll.scrollRef,
              },
            },
              h("div", { ref: scroll.feedRef, style: { height: 1000, flexShrink: 0 } }, "feed"),
            ),
          ),
          h(ConversationPanelBottomArea, {
            activity: h("div", {
              "data-test-activity": true,
              style: { height: activityHeight, width: "100%" },
            }),
            children: h("div", { style: { height: 80 } }),
            goal: null,
            isMobileLayout: true,
            isReconciling: false,
            onReconcile: () => undefined,
            providerWarningVisible: false,
            reliability: {
              failure: null,
              provider_retry: null,
              transport_phase: "healthy",
            },
            scrollToLatest: {
              isGenerating: false,
              onClick: () => undefined,
              visible: true,
            },
          }),
          h("button", {
            onClick: () => setActivityHeight((current: number) => current === 46 ? 190 : 46),
          }, "Toggle wrapped activity"),
          h("button", {
            onClick: () => {
              const element = scroll.scrollRef.current;
              if (element) {
                element.scrollTop -= 300;
                scroll.pauseFollowLatest();
              }
            },
          }, "Read earlier"),
        ),
      );
    }
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0;z-index:99999;background:white";
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });
  const viewport = page.locator(".overflow-y-auto").last();
  const geometry = () => viewport.evaluate((element) => ({
    bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
    clearance: element.querySelector("[data-conversation-dock-clearance]")?.getBoundingClientRect().height ?? 0,
    scrollTop: element.scrollTop,
  }));
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  await page.getByRole("button", { name: "Toggle wrapped activity" }).click();
  await expect.poll(async () => (await geometry()).clearance).toBe(198);
  expect((await geometry()).bottomGap).toBe(0);

  await page.getByRole("button", { name: "Read earlier" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(300);
  const readingTop = await viewport.evaluate((element) => element.scrollTop);
  await page.getByRole("button", { name: "Toggle wrapped activity" }).click();
  await expect.poll(async () => (await geometry()).clearance).toBe(56);
  expect(await viewport.evaluate((element) => element.scrollTop)).toBe(readingTop);
  expect((await geometry()).bottomGap).toBe(158);
});

test("conversation width reflow keeps the same reading anchor while a right panel is resized", async ({ page }) => {
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
      const [width, setWidth] = React.useState(700);
      const scroll = useFollowScroll({
        contentKey: "fixed-width-content",
        messageCount: 30,
        sessionKey: "width-resize-test",
        topologyKey: "fixed-width-topology",
      });
      return h("div", { style: { height: "100%", width: "100%" } },
        h("button", {
          onClick: () => setWidth((current: number) => current === 700 ? 320 : 700),
        }, "Toggle conversation width"),
        h("button", {
          onClick: () => {
            const element = scroll.scrollRef.current;
            if (element) {
              element.scrollTop -= 4000;
              scroll.pauseFollowLatest();
            }
          },
        }, "Read earlier"),
        h("div", {
          "data-width-scroll": true,
          ref: scroll.scrollRef,
          onScroll: scroll.onScroll,
          onWheel: scroll.onWheel,
          style: {
            height: 500,
            overflowAnchor: "none",
            overflowY: "auto",
            width,
          },
        },
          h("div", {
            ref: scroll.feedRef,
            style: { display: "flex", flexDirection: "column", width: "100%" },
          },
            Array.from({ length: 30 }, (_, index) => h("div", {
              "data-conversation-round-id": String(index),
              key: index,
              style: {
                fontSize: 16,
                lineHeight: "24px",
                overflowWrap: "anywhere",
                padding: "10px 0",
              },
            }, `Round ${index} ${"long content that reflows when the conversation width changes ".repeat(10)}`)),
          ),
        ),
      );
    }
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0;z-index:99999;background:white";
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });
  const viewport = page.locator("[data-width-scroll]");
  const geometry = (targetId?: string | null) => viewport.evaluate((element, targetRoundId) => {
    const viewportRect = element.getBoundingClientRect();
    const anchor = Array.from(element.querySelectorAll<HTMLElement>("[data-conversation-round-id]"))
      .find((round) => {
        const rect = round.getBoundingClientRect();
        return rect.bottom > viewportRect.top && rect.top < viewportRect.bottom;
      });
    const target = targetRoundId
      ? element.querySelector<HTMLElement>(`[data-conversation-round-id="${targetRoundId}"]`)
      : null;
    return {
      anchorId: anchor?.dataset.conversationRoundId ?? null,
      anchorTop: anchor ? anchor.getBoundingClientRect().top - viewportRect.top : null,
      bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
      targetTop: target ? target.getBoundingClientRect().top - viewportRect.top : null,
    };
  }, targetId);
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  await page.getByRole("button", { name: "Toggle conversation width" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  await page.getByRole("button", { name: "Read earlier" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBeGreaterThan(2000);
  const beforeResize = await geometry();
  await page.getByRole("button", { name: "Toggle conversation width" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBeGreaterThan(0);
  const afterResize = await geometry(beforeResize.anchorId);
  expect(afterResize.targetTop).not.toBeNull();
  expect(Math.abs(afterResize.targetTop! - beforeResize.anchorTop!)).toBeLessThan(1);
});

for (const reading of [false, true]) {
  test(`${reading ? "READING" : "FOLLOW"} survives the static to virtual Feed handoff`, async ({ page }) => {
    await page.goto("/ui-gallery.html");
    await page.evaluate(async () => {
      const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
      const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
      const virtualPath = "/node_modules/.vite-browser-test/deps/@tanstack_react-virtual.js";
      const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
      const policyPath = "/src/features/conversation/shared/feed/use-conversation-virtualization-policy.ts";
      const { default: React } = await import(reactPath);
      const { default: { createRoot } } = await import(domPath);
      const { useVirtualizer } = await import(virtualPath);
      const { useFollowScroll } = await import(hookPath);
      const { useConversationVirtualizationPolicy } = await import(policyPath);
      const h = React.createElement;
      function Harness() {
        const [count, setCount] = React.useState(19);
        const [active, setActive] = React.useState(true);
        const scroll = useFollowScroll({
          contentKey: String(count),
          liveLayoutActive: active,
          messageCount: count,
          sessionKey: "static-virtual-handoff",
          topologyKey: String(count),
        });
        const virtualEnabled = useConversationVirtualizationPolicy({
          active,
          count,
          scopeKey: "static-virtual-handoff",
          threshold: 20,
        });
        const virtualizer = useVirtualizer({
          count,
          estimateSize: () => 120,
          getScrollElement: () => scroll.scrollRef.current,
          measureElement: (element: HTMLElement) => element.getBoundingClientRect().height,
          overscan: 2,
        });
        const finish = () => {
          setCount(20);
          setActive(false);
        };
        const rows = (indices: number[]) => indices.map((index) => h("div", {
          "data-conversation-round-id": String(index),
          key: index,
          ref: virtualEnabled ? virtualizer.measureElement : undefined,
          style: { height: 120, flexShrink: 0 },
        }, `Round ${index}`));
        return h("div", { style: { position: "fixed", inset: 0, zIndex: 99999, background: "white" } },
          h("button", { onClick: finish }, "Finish response"),
          h("button", {
            onClick: () => {
              const element = scroll.scrollRef.current;
              if (element) {
                element.scrollTop -= 240;
                scroll.pauseFollowLatest();
              }
            },
          }, "Read earlier"),
          h("div", {
            "data-handoff-mode": virtualEnabled ? "virtual" : "static",
            "data-handoff-scroll": true,
            ref: scroll.scrollRef,
            onScroll: scroll.onScroll,
            onWheel: scroll.onWheel,
            style: { height: 500, overflowY: "auto", overflowAnchor: "none" },
          },
            virtualEnabled
              ? h("div", {
                ref: scroll.feedRef,
                "data-conversation-virtual-feed": "true",
                style: { position: "relative", height: virtualizer.getTotalSize() },
              }, virtualizer.getVirtualItems().map((item: { index: number; key: string | number; start: number }) => h("div", {
                key: item.key,
                "data-conversation-round-id": String(item.index),
                ref: virtualizer.measureElement,
                style: {
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${item.start}px)`,
                  height: 120,
                },
              }, `Round ${item.index}`)))
              : h("div", {
                ref: scroll.feedRef,
                style: { display: "flex", flexDirection: "column" },
              }, rows(Array.from({ length: count }, (_, index) => index))),
          ),
        );
      }
      const host = document.createElement("div");
      document.body.append(host);
      createRoot(host).render(h(Harness));
    });
    const viewport = page.locator("[data-handoff-scroll]");
    const mode = () => page.locator("[data-handoff-mode]").getAttribute("data-handoff-mode");
    const geometry = (roundId?: string | null) => viewport.evaluate((element, targetId) => {
      const viewportRect = element.getBoundingClientRect();
      const target = targetId
        ? element.querySelector<HTMLElement>(`[data-conversation-round-id="${targetId}"]`)
        : null;
      return {
        bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
        scrollTop: element.scrollTop,
        targetTop: target ? target.getBoundingClientRect().top - viewportRect.top : null,
      };
    }, roundId);
    await expect.poll(mode).toBe("static");
    await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
    if (reading) {
      await page.getByRole("button", { name: "Read earlier" }).click();
      await expect.poll(async () => (await geometry()).bottomGap).toBe(240);
    }
    const before = await geometry(reading ? "16" : null);
    await page.getByRole("button", { name: "Finish response" }).click();
    await expect.poll(mode).toBe("virtual");
    await expect.poll(async () => (await geometry()).bottomGap).toBe(reading ? 360 : 0);
    const after = await geometry("16");
    if (reading) {
      expect(after.targetTop).not.toBeNull();
      expect(Math.abs(after.targetTop! - before.targetTop!)).toBeLessThan(1);
      expect(after.scrollTop).toBe(before.scrollTop);
    } else {
      expect(after.scrollTop).toBeGreaterThan(0);
    }
  });
}

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
  const bottomTop = (await geometry()).top;
  await page.getByTestId("scroll").hover();
  await page.mouse.wheel(0, -200);
  // Linux WebKit animates native wheel input. Capture the reading position only
  // after the requested movement finishes, not at its first intermediate frame.
  await expect.poll(async () => (await geometry()).top).toBe(bottomTop - 200);
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

test("expanded thought headers cover scrolling detail text", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const thoughtPath = "/src/features/conversation/shared/message/blocks/thinking-block.tsx";
    const railPath = "/src/features/conversation/shared/message/ui/message-rail.tsx";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { ThinkingBlock } = await import(thoughtPath);
    const { MessageDetailScroll } = await import(railPath);
    const { I18nProvider } = await import(i18nPath);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:30px;z-index:99999;background:var(--background)";
    document.body.append(host);
    createRoot(host).render(React.createElement(I18nProvider, null,
      React.createElement(MessageDetailScroll, null, React.createElement(ThinkingBlock, {
        defaultExpanded: true, thinking: "Long thought content that must not show through the sticky title.\n\n".repeat(60),
      }))));
  });
  const header = page.locator('[data-message-detail-sticky-header="true"]');
  const scroll = page.locator('[data-message-detail-scroll]').last();
  await expect(header).toBeVisible();
  await scroll.evaluate((element) => { element.scrollTop = 120; });
  const headerTop = (await header.boundingBox())!.y;
  expect(Math.abs(headerTop - (await scroll.boundingBox())!.y)).toBeLessThan(1);
  // 展开、悬浮和按下态都必须遮住已经滚过标题的正文。
  const background = () => header.evaluate((element) => getComputedStyle(element).backgroundColor);
  expect(await background()).not.toBe("rgba(0, 0, 0, 0)");
  await header.hover();
  expect(await background()).not.toBe("rgba(0, 0, 0, 0)");
  await page.mouse.down();
  expect(await background()).not.toBe("rgba(0, 0, 0, 0)");
  await page.mouse.move(0, 0);
  await page.mouse.up();
});
