// INPUT: Production FOLLOW hook and real browser layout.
// OUTPUT: Live process shrink/growth, terminal/reading geometry and bottom-control visibility regressions.
// POS: Isolated conversation scroll harness; no model or backend requests.
import { expect, test } from "@playwright/test";

test("scroll control floats over the feed and reaching the bottom moves nothing", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
    const layoutPath = "/src/features/conversation/shared/conversation-panel-layout.tsx";
    const modelPath = "/src/features/conversation/shared/conversation-panel-model.ts";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { I18nProvider } = await import(i18nPath);
    const { useFollowScroll } = await import(hookPath);
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
      const scroll = useFollowScroll({
        contentKey: "fixed",
        messageCount: 1,
        sessionKey: "scroll-control-test",
        topologyKey: "fixed",
      });
      const frame = buildConversationPanelFrameModel({
        conversation: {
          is_history_loading: false,
          is_loading: isLoading,
          is_session_loading: false,
          load_round_window: () => undefined,
          load_session: () => undefined,
          reliability: healthyReliability,
        },
        history: { handleScroll: scroll.onScroll },
        roundIndexResource: {
          access: false,
          error: null,
          isLoading: false,
          isStale: false,
          retry: () => undefined,
        },
        roundScrollRef: { current: null },
        scroll,
        sessionKey: "scroll-control-test",
        timeline: {},
      }, { isMobileLayout: false, providerWarningVisible: false }, {});
      return h(I18nProvider, null,
        h(ConversationPanelLayout, null,
          h(ConversationPanelViewportArea, null,
            h(ConversationPanelViewport, {
              isMobileLayout: false,
              viewport: frame.viewport,
            }, h("div", { ref: scroll.feedRef, style: { height: 2000, flexShrink: 0 } }, "feed")),
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
            onClick: () => setIsLoading((value: boolean) => !value),
          }, "Toggle generation"),
        ),
      );
    }
    const host = document.createElement("div");
    host.dataset.scrollControlHarness = "true";
    host.style.cssText = "position:fixed;inset:0;z-index:99999;background:white";
    document.body.append(host);
    createRoot(host).render(h(Harness));
  });

  const harness = page.locator("[data-scroll-control-harness]");
  const control = harness.locator("[data-scroll-to-latest]");
  const clearance = harness.locator("[data-conversation-dock-clearance]");
  const viewport = harness.locator(".overflow-y-auto").last();
  const geometry = () => viewport.evaluate((element) => ({
    bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
    scrollHeight: element.scrollHeight,
  }));
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
  await expect(control).toHaveCount(0);
  const { scrollHeight } = await geometry();

  // The control appears over the feed: it reserves no tail clearance.
  await viewport.hover();
  await page.mouse.wheel(0, -300);
  await expect(control).toHaveCount(1);
  await expect(control).toHaveAttribute("data-generating", "true");
  await expect(clearance).toHaveCount(0);
  expect(await geometry()).toEqual({ bottomGap: 300, scrollHeight });

  await page.getByRole("button", { name: "Toggle generation" }).click();
  await expect(control).not.toHaveAttribute("data-generating", "true");

  // Arriving hides the control without changing the scroll content, so the
  // tail the reader scrolled to neither slides down nor keeps a blank gap.
  await viewport.hover();
  await page.mouse.wheel(0, 400);
  await expect(control).toHaveCount(0);
  await expect(clearance).toHaveCount(0);
  expect(await geometry()).toEqual({ bottomGap: 0, scrollHeight });
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

test("Goal clearance keeps the latest message above the floating Goal while FOLLOW reaches the real bottom", async ({ page }) => {
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
    const { useFollowScroll } = await import(hookPath);
    const {
      ConversationPanelBottomArea,
      ConversationPanelLayout,
      ConversationPanelViewport,
      ConversationPanelViewportArea,
    } = await import(layoutPath);
    const h = React.createElement;
    const healthyReliability = {
      failure: null,
      provider_retry: null,
      transport_phase: "healthy",
    };

    function Harness() {
      const [showGoal, setShowGoal] = React.useState(false);
      const [goalHeight, setGoalHeight] = React.useState(96);
      const scroll = useFollowScroll({
        contentKey: "goal-clearance",
        messageCount: 1,
        sessionKey: "goal-clearance-test",
        topologyKey: "goal-clearance",
      });
      return h(I18nProvider, null,
        h(ConversationPanelLayout, null,
          h(ConversationPanelViewportArea, null,
            h(ConversationPanelViewport, {
              floatingDockOccupied: false,
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
              h("div", { ref: scroll.feedRef, style: { height: 1800, flexShrink: 0 } }, "feed"),
            ),
          ),
          h(ConversationPanelBottomArea, {
            children: h("div", { style: { height: 80 } }),
            goal: showGoal
              ? h("div", {
                  "data-test-goal": true,
                  style: { height: goalHeight, width: "100%" },
                }, "Goal")
              : null,
            isMobileLayout: false,
            isReconciling: false,
            onReconcile: () => undefined,
            providerWarningVisible: false,
            reliability: healthyReliability,
            scrollToLatest: { isGenerating: false, onClick: () => undefined, visible: false },
          }),
          h("button", { onClick: () => setShowGoal((value: boolean) => !value) }, "Toggle Goal"),
          h("button", { onClick: () => setGoalHeight(180) }, "Grow Goal"),
          h("button", {
            onClick: () => {
              const element = scroll.scrollRef.current;
              if (element) {
                element.scrollTop -= 240;
                scroll.pauseFollowLatest();
              }
            },
          }, "Read earlier"),
          h("button", { onClick: () => scroll.scrollToBottom("auto") }, "Go latest"),
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
  await page.getByRole("button", { name: "Toggle Goal" }).click();
  await expect.poll(async () => (await geometry()).clearance).toBeGreaterThan(0);
  expect((await geometry()).bottomGap).toBe(0);

  await page.getByRole("button", { name: "Read earlier" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(240);
  const readingTop = await viewport.evaluate((element) => element.scrollTop);
  await page.getByRole("button", { name: "Grow Goal" }).click();
  await expect.poll(async () => (await geometry()).clearance).toBeGreaterThan(0);
  expect(await viewport.evaluate((element) => element.scrollTop)).toBe(readingTop);

  await page.getByRole("button", { name: "Go latest" }).click();
  await expect.poll(async () => (await geometry()).bottomGap).toBe(0);
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

test("process icons stay aligned when collapsed and expanded", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const runsPath = "/src/features/conversation/shared/message/item/view/assistant/assistant-dm-tool-runs.tsx";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { AssistantToolRuns } = await import(runsPath);
    const { I18nProvider } = await import(i18nPath);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:20px;z-index:99999;background:var(--background)";
    document.body.append(host);
    createRoot(host).render(React.createElement(I18nProvider, null,
      React.createElement(AssistantToolRuns, {
        activity: { emptyStreamStatus: null, label: null, showCursor: true, standalone: false, state: "thinking", toolUseSummary: null },
        environment: { mode: "dm_live", hiddenToolNames: [], canRespondToPermissions: false },
        permissions: { all: [], matchedByToolUseId: new Map(), owner: "composer", unmatched: [] },
        projection: { content: [
          { type: "text", text: "Alignment paragraph" },
          { type: "thinking", thinking: "Inspect source" },
          { type: "tool_use", id: "align-read", name: "Read", input: { file_path: "a.md" } },
        ], streamingIndexes: new Set() },
        responseResumed: false,
      })));
  });
  const group = page.locator('[data-tool-run-id]').last();
  const toggle = group.locator('button[aria-expanded]').first();
  const stackIcon = group.locator('[data-process-activity-icon] svg').first();
  await expect(stackIcon).toBeVisible();
  const stack = (await group.locator("[data-process-activity-icon-stack]").boundingBox())!;
  const paragraph = (await page.getByText("Alignment paragraph", { exact: true }).boundingBox())!;
  expect(Math.abs(stack.x - paragraph.x)).toBeLessThan(1);
  const collapsed = (await stackIcon.boundingBox())!;
  const thinking = (await group.locator('[data-message-activity-icon] svg').boundingBox())!;
  expect(Math.abs(collapsed.x - thinking.x)).toBeLessThan(1);
  expect(collapsed.width).toBe(thinking.width);
  await toggle.click();
  const expanded = (await toggle.locator('svg').first().boundingBox())!;
  expect(Math.abs(collapsed.x - expanded.x)).toBeLessThan(1);
  expect(collapsed.width).toBe(expanded.width);
  await toggle.click();
  expect(Math.abs((await stackIcon.boundingBox())!.x - collapsed.x)).toBeLessThan(1);
});

test("conversation delete actions replace the date meta slot", async ({ page }) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const rowPath = "/src/features/home/sidebar/sidebar-list-rows.tsx";
    const i18nPath = "/src/shared/i18n/i18n-provider.tsx";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { ConversationRow } = await import(rowPath);
    const { I18nProvider } = await import(i18nPath);
    const host = document.createElement("div");
    host.dataset.deleteAlignment = "true";
    host.style.cssText = "position:fixed;inset:20px;max-width:300px;z-index:99999;background:var(--background)";
    document.body.append(host);
    createRoot(host).render(React.createElement(I18nProvider, null,
      ...["person", "dm", "room"].map((kind) => React.createElement(ConversationRow, {
        key: kind, isActive: false, onClick: () => {}, onDelete: () => {},
        item: { id: kind, kind: kind === "person" ? "dm" : kind, directUserId: kind === "person" ? "user" : undefined,
          title: kind, summary: "Conversation preview", timeLabel: "12:30", members: [],
          isPinned: false, lastActivityAt: 0, messageCount: 1, activityStatus: kind === "room" ? "waiting" : "working", unreadCount: 2, canDelete: true },
      }))));
  });
  const rows = page.locator('[data-delete-alignment] .group\\/item');
  await expect(rows).toHaveCount(3);
  for (const row of await rows.all()) {
    const timeLabel = row.getByText("12:30");
    await page.mouse.move(0, 0);
    await expect(timeLabel).toHaveCSS("opacity", "1");
    await row.hover();
    const action = row.locator("button");
    await expect(action).toHaveCSS("opacity", "1");
    await expect(timeLabel).toHaveCSS("opacity", "0");
    await page.mouse.move(0, 0);
    await expect(timeLabel).toHaveCSS("opacity", "1");
    await action.focus();
    await expect(timeLabel).toHaveCSS("opacity", "0");
    await action.evaluate((element) => element.blur());
    const bounds = (await row.boundingBox())!;
    const button = (await action.boundingBox())!;
    const time = (await timeLabel.boundingBox())!;
    expect(Math.abs(button.y + button.height / 2 - time.y - time.height / 2)).toBeLessThan(1);
    expect(button.x).toBeGreaterThan(bounds.x + bounds.width / 2);
    expect(button.x + button.width).toBeLessThanOrEqual(bounds.x + bounds.width);
  }
});

// handoff：与真实长会话一致，首屏是静态 Feed，上滚途中驻留轮次越过阈值才切换为虚拟 Feed。
for (const renderer of ["static", "virtual", "handoff"] as const) {
  test(`${renderer} continuous window keeps the reading position through prepend, eviction and reload`, async ({ page }) => {
    test.setTimeout(90_000);
    await page.goto("/ui-gallery.html");
    await page.evaluate(async (renderer) => {
      const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
      const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
      const virtualPath = "/node_modules/.vite-browser-test/deps/@tanstack_react-virtual.js";
      const hookPath = "/src/features/conversation/shared/timeline/scroll/use-follow-scroll.ts";
      const loaderPath = "/src/features/conversation/shared/timeline/window-loader/use-continuous-window-loader.ts";
      const policyPath = "/src/features/conversation/shared/feed/use-conversation-virtual-scroll-policy.ts";
      const controllerPath = "/src/features/conversation/shared/timeline/scroll/scroll-controller.ts";
      const canvasPath = "/src/features/conversation/shared/feed/conversation-virtual-canvas.tsx";
      const virtualizationPath = "/src/features/conversation/shared/feed/use-conversation-virtualization-policy.ts";
      const { default: React } = await import(reactPath);
      const { default: { createRoot } } = await import(domPath);
      const { useVirtualizer } = await import(virtualPath);
      const { useFollowScroll } = await import(hookPath);
      const { useContinuousWindowLoader } = await import(loaderPath);
      const { ConversationVirtualCanvas } = await import(canvasPath);
      const { useConversationVirtualizationPolicy } = await import(virtualizationPath);
      const {
        scrollConversationVirtualElement,
        shouldAdjustConversationVirtualScrollPosition,
        useConversationVirtualHandoff,
        useConversationVirtualRemeasure,
        useConversationVirtualTopologyAnchor,
      } = await import(policyPath);
      const { getConversationScrollWriteConflicts } = await import(controllerPath);
      const h = React.createElement;
      const TOTAL = 160;
      const CAP = 40;
      // 与真实会话一致：一页只有几轮，实测高度远大于虚拟估高。
      const PAGE = 3;
      const VIRTUAL_THRESHOLD = { handoff: 30, static: Infinity, virtual: 0 };
      const heightOf = (index: number) => 220 + (index % 5) * 90;
      const wait = () => new Promise((resolve) => setTimeout(resolve, 30));
      type RoundWindow = { first: number; last: number };

      type FeedProps = {
        indexes: number[];
        roundIds: string[];
        scroll: ReturnType<typeof useFollowScroll>;
      };
      const roundStyle = (index: number) => ({
        boxSizing: "border-box",
        borderBottom: "1px solid #ddd",
        flexShrink: 0,
        height: heightOf(index),
      });

      function StaticFeed({ indexes, scroll }: FeedProps) {
        return h("div", {
          ref: scroll.feedRef,
          style: { display: "flex", flexDirection: "column" },
        }, indexes.map((index, position) => h("div", {
          "data-conversation-round-id": String(index),
          "data-conversation-round-index": position,
          key: index,
          style: roundStyle(index),
        }, `Round ${index}`)));
      }

      function VirtualFeed({ indexes, roundIds, scroll }: FeedProps) {
        const shouldHoldReading = () => !scroll.isFollowingLatest() && !scroll.isBottomScrollActive();
        const handoff = useConversationVirtualHandoff(scroll.scrollRef, {
          estimateSize: () => 60,
          getItemKey: (index: number) => roundIds[index],
          roundIds,
        }, shouldHoldReading);
        const virtualizer = useVirtualizer({
          count: indexes.length,
          estimateSize: () => 60,
          getItemKey: (index: number) => roundIds[index],
          getScrollElement: () => scroll.scrollRef.current,
          initialOffset: handoff.initialOffset,
          measureElement: (element: HTMLElement) => element.getBoundingClientRect().height,
          overscan: 2,
          scrollToFn: scrollConversationVirtualElement,
        });
        const getTopologyAnchorIndex = useConversationVirtualTopologyAnchor(
          virtualizer,
          roundIds,
          shouldHoldReading,
          handoff.readingAnchor,
        );
        useConversationVirtualRemeasure(virtualizer, "typography");
        virtualizer.shouldAdjustScrollPositionOnItemSizeChange = (
          item: { end: number; index: number },
          delta: number,
          instance: unknown,
        ) => shouldAdjustConversationVirtualScrollPosition(item, delta, instance, {
          bottomScrollActive: scroll.isBottomScrollActive(),
          followingLatest: scroll.isFollowingLatest(),
          navigationActive: false,
          topologyAnchorIndex: getTopologyAnchorIndex(),
          userScrollActive: scroll.isUserScrollActive(),
        });
        // 与真实 Feed 相同的画布：已挂载的行按真实高度流式排布，整体平移到首个 item 的起点。
        const items: Array<{ index: number; start: number }> = virtualizer.getVirtualItems();
        const totalSize = virtualizer.getTotalSize();
        return h("div", {
          ref: scroll.feedRef,
          "data-conversation-virtual-feed": "true",
          style: { position: "relative", height: totalSize },
        }, h(ConversationVirtualCanvas, { offset: items[0]?.start ?? 0, totalSize }, items.map((item) => h("div", {
          "data-conversation-round-id": String(indexes[item.index]),
          "data-conversation-round-index": item.index,
          "data-index": item.index,
          key: indexes[item.index],
          ref: virtualizer.measureElement,
          style: roundStyle(indexes[item.index]),
        }, `Round ${indexes[item.index]}`))));
      }

      let isFollowingLatest = () => false;
      function Harness() {
        // 与真实会话一致：滚动容器先挂载，首屏历史随后异步到达。
        const [loaded, setLoaded] = React.useState(false);
        React.useEffect(() => setLoaded(true), []);
        const [roundWindow, setRoundWindow] = React.useState({ first: TOTAL - 20, last: TOTAL - 1 });
        const windowRef = React.useRef(roundWindow);
        windowRef.current = roundWindow;
        const indexes = React.useMemo(() => (loaded ? Array.from(
          { length: roundWindow.last - roundWindow.first + 1 },
          (_, offset) => roundWindow.first + offset,
        ) : []), [loaded, roundWindow]);
        const roundIds = React.useMemo(() => indexes.map(String), [indexes]);
        const revision = loaded ? `${roundWindow.first}:${roundWindow.last}` : "loading";
        const scroll = useFollowScroll({
          contentKey: revision,
          detachedFromLatest: loaded && roundWindow.last < TOTAL - 1,
          messageCount: indexes.length,
          sessionKey: "continuous-window",
          topologyKey: revision,
        });
        isFollowingLatest = scroll.isFollowingLatest;
        const commit = React.useCallback(async (next: (current: RoundWindow) => RoundWindow) => {
          await wait();
          const current = windowRef.current;
          const result = next(current);
          setRoundWindow(result);
          return result.first !== current.first || result.last !== current.last;
        }, []);
        useContinuousWindowLoader({
          enabled: loaded,
          feedRef: scroll.feedRef,
          gapAfterRoundId: roundWindow.last < TOTAL - 1 ? String(roundWindow.last) : null,
          hasOlderHistory: roundWindow.first > 0,
          isBottomScrollActive: scroll.isBottomScrollActive,
          isFollowingLatest: scroll.isFollowingLatest,
          isSessionLoading: false,
          // 向前扩展从最新一端淘汰，向后扩展从最旧一端淘汰。
          loadNewerMessages: () => commit(({ last }: RoundWindow) => {
            const nextLast = Math.min(TOTAL - 1, last + PAGE);
            return { first: Math.max(windowRef.current.first, nextLast - CAP + 1), last: nextLast };
          }),
          loadOlderMessages: () => commit(({ first, last }: RoundWindow) => {
            const nextFirst = Math.max(0, first - PAGE);
            return { first: nextFirst, last: Math.min(last, nextFirst + CAP - 1) };
          }),
          returnToLatestMessages: () => commit(() => ({ first: TOTAL - 20, last: TOTAL - 1 })),
          revision,
          scopeKey: "continuous-window",
          scrollRef: scroll.scrollRef,
          setEvictionEdge: () => undefined,
          setWindowChangePending: scroll.setWindowChangePending,
          windowRoundIds: roundIds,
        });
        const feed = { indexes, roundIds, scroll };
        const virtualEnabled = useConversationVirtualizationPolicy({
          active: false,
          count: indexes.length,
          scopeKey: "window",
          threshold: VIRTUAL_THRESHOLD[renderer],
        });
        return h("div", { style: { position: "fixed", inset: 0, zIndex: 99999, background: "white" } },
          h("div", {
            "data-window-first": roundWindow.first,
            "data-window-last": roundWindow.last,
            "data-window-scroll": true,
            ref: scroll.scrollRef,
            onPointerDown: scroll.onPointerDown,
            onScroll: scroll.onScroll,
            onWheel: scroll.onWheel,
            style: { height: 500, overflowY: "auto", overflowAnchor: "none" },
            "data-window-renderer": virtualEnabled ? "virtual" : "static",
          }, loaded ? h(virtualEnabled ? VirtualFeed : StaticFeed, feed) : null),
        );
      }
      const host = document.createElement("div");
      document.body.append(host);
      createRoot(host).render(h(Harness));

      // 每帧记录视口顶端在完整会话中的绝对位置：窗口怎样增删都不应让它跳变。
      const offsets = [0];
      for (let index = 0; index < TOTAL; index += 1) {
        offsets.push(offsets[index] + heightOf(index));
      }
      const probe = {
        conflicts: () => [] as unknown[],
        following: () => isFollowingLatest(),
        samples: [] as number[],
      };
      (window as unknown as { __continuousWindow: typeof probe }).__continuousWindow = probe;
      const sample = () => {
        const viewport = document.querySelector<HTMLElement>("[data-window-scroll]");
        if (viewport) {
          probe.conflicts = () => [...getConversationScrollWriteConflicts(viewport)];
          const top = viewport.getBoundingClientRect().top;
          const first = [...viewport.querySelectorAll<HTMLElement>("[data-conversation-round-id]")]
            .map((element) => ({ element, rect: element.getBoundingClientRect() }))
            .filter(({ rect }) => rect.bottom > top + 1)
            .sort((left, right) => left.rect.top - right.rect.top)[0];
          if (first && first.rect.top <= top + 1) {
            probe.samples.push(
              offsets[Number(first.element.dataset.conversationRoundId)] + top - first.rect.top,
            );
          }
        }
        requestAnimationFrame(sample);
      };
      requestAnimationFrame(sample);
    }, renderer);

    const viewport = page.locator("[data-window-scroll]");
    const state = () => page.evaluate(() => {
      const element = document.querySelector<HTMLElement>("[data-window-scroll]")!;
      const probe = (window as unknown as {
        __continuousWindow: { conflicts: () => unknown[]; following: () => boolean; samples: number[] };
      }).__continuousWindow;
      return {
        bottomGap: element.scrollHeight - element.clientHeight - element.scrollTop,
        conflicts: probe.conflicts(),
        first: Number(element.dataset.windowFirst),
        following: probe.following(),
        last: Number(element.dataset.windowLast),
        samples: probe.samples.splice(0),
        scrollTop: element.scrollTop,
      };
    });
    const wheelUntil = async (deltaY: number, done: (current: Awaited<ReturnType<typeof state>>) => boolean) => {
      const samples: number[] = [];
      for (let step = 0; step < 600; step += 1) {
        // 触控板式的连续小步滚动，中间不等待加载完成。
        await page.mouse.wheel(0, deltaY);
        await page.mouse.wheel(0, deltaY);
        await page.waitForTimeout(16);
        const current = await state();
        samples.push(...current.samples);
        if (done(current)) {
          return { current, samples };
        }
      }
      throw new Error("the reader never reached the requested edge");
    };

    await expect.poll(async () => {
      const current = await state();
      return current.scrollTop > 0 && current.bottomGap <= 1;
    }).toBe(true);
    await viewport.hover();
    await state();

    const up = await wheelUntil(-150, (current) => current.first === 0 && current.scrollTop === 0);
    expect(up.current.last).toBe(39);
    // 静态 Feed 的最后一次前插与切换到 Virtualizer 可能落在同一帧，这是一次所有权交接。
    const isHandoff = (conflict: unknown) => {
      const { first, second } = conflict as { first: string; second: string };
      return renderer === "handoff" && first === "anchor" && second === "virtualizer";
    };
    expect(up.current.conflicts.filter((conflict) => !isHandoff(conflict))).toEqual([]);
    expect(up.current.conflicts.length).toBeLessThanOrEqual(1);
    await expect(viewport).toHaveAttribute("data-window-renderer", renderer === "static" ? "static" : "virtual");
    for (let index = 1; index < up.samples.length; index += 1) {
      const moved = up.samples[index] - up.samples[index - 1];
      expect(moved, `upward frame ${index}`).toBeLessThanOrEqual(2);
      expect(moved, `upward frame ${index}`).toBeGreaterThan(-700);
    }

    // 一步滚到驻留窗口的底部，赶在下一页到达之前。这里只是断点而不是会话底部：
    // 新页提交时读者留在原处，不会被 FOLLOW 一页页带向最新。
    await page.mouse.wheel(0, 1_000_000);
    await expect.poll(async () => (await state()).last).toBe(42);
    await state();
    await page.waitForTimeout(600);
    const rested = await state();
    expect(rested.last).toBe(42);
    expect(rested.following).toBe(false);
    expect(Math.max(...rested.samples) - Math.min(...rested.samples)).toBeLessThanOrEqual(2);

    const down = await wheelUntil(150, (current) => current.last === 159 && current.bottomGap <= 1);
    expect(down.current.first).toBe(120);
    expect(down.current.following).toBe(true);
    for (let index = 1; index < down.samples.length; index += 1) {
      const moved = down.samples[index] - down.samples[index - 1];
      expect(moved, `downward frame ${index}`).toBeGreaterThanOrEqual(-2);
      expect(moved, `downward frame ${index}`).toBeLessThan(700);
    }
    expect(down.current.conflicts).toEqual(up.current.conflicts);
  });
}
