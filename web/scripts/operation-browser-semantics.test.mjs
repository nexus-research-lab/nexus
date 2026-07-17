import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

const {
  buildBrowserAgentPages,
  createBrowserUserPage,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/browser-page-model.ts",
);
const {
  activeBrowserPage,
  activeBrowserTab,
  createBrowserNavigationState,
  moveBrowserHistory,
  navigateBrowserAddress,
  openBrowserTab,
  reloadBrowserPage,
  syncBrowserAgentPages,
  toggleBrowserReader,
} = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/browser-navigation-model.ts",
);
const { BrowserSurface } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/browser-surface.tsx",
);
const { readBrowserPageBridgeMessage } = await server.ssrLoadModule(
  "/src/features/conversation/operation/apps/browser-page-bridge.ts",
);

const now = Date.now();

function proxyUrl(target) {
  return `/nexus/v1/operation/browser/page?${new URLSearchParams({ url: target }).toString()}`;
}

function webEvent({
  id,
  input,
  phase = "done",
  result,
  target,
  toolName,
  timestamp,
}) {
  return {
    id,
    session_key: "session:browser-test",
    round_id: "round:browser-test",
    agent_id: "agent:test",
    message_id: `message:${id}`,
    tool_use_id: `tool:${id}`,
    tool_name: toolName,
    kind: "web_research",
    surface: "web",
    phase,
    title: toolName === "WebSearch" ? "搜索网页" : "抓取网页",
    target,
    input_preview: input,
    result_preview: result,
    started_at: timestamp,
    updated_at: timestamp,
  };
}

function buildToolPages() {
  const search = webEvent({
    id: "search",
    input: { query: "Nexus operation stage" },
    result: [{
      title: "Nexus stage architecture",
      url: "https://example.com/stage",
      snippet: "A persistent browser session for agent operations.",
    }],
    target: "Nexus operation stage",
    toolName: "WebSearch",
    timestamp: now,
  });
  const fetch = webEvent({
    id: "fetch",
    input: {
      prompt: "Find the browser session behavior.",
      url: "https://example.com/stage",
    },
    result: {
      content: "The browser keeps navigation history after the agent run.\nUsers can revisit fetched pages.",
      is_error: false,
    },
    target: "https://example.com/stage",
    toolName: "WebFetch",
    timestamp: now + 1,
  });
  return {
    fetch,
    pages: buildBrowserAgentPages({
      event: fetch,
      preview: fetch.result_preview,
      query: "https://example.com/stage",
      related_events: [search, fetch],
      target: fetch.target,
      web_url_builder: proxyUrl,
    }),
    search,
  };
}

test("WebSearch and WebFetch become one navigable agent history", () => {
  const { pages } = buildToolPages();

  assert.equal(pages.length, 2);
  assert.equal(pages[0].kind, "search");
  assert.equal(pages[0].results[0].url, "https://example.com/stage");
  assert.equal(pages[1].kind, "web");
  assert.equal(pages[1].address, "https://example.com/stage");
  assert.equal(pages[1].iframe_url, proxyUrl("https://example.com/stage"));
  assert.equal(pages[1].presentation, "live");
  assert.equal(pages[1].reader.url, "https://example.com/stage");
  assert.ok(pages[1].reader.paragraphs.some((paragraph) => paragraph.text.includes("navigation history")));
});

test("WebSearch tool semantics win when the query itself looks like a URL", () => {
  const search = webEvent({
    id: "url-query-search",
    input: { query: "https://example.com/reference" },
    result: [{
      title: "Reference result",
      url: "https://example.com/reference",
      snippet: "A search result for a URL-shaped query.",
    }],
    target: "https://example.com/reference",
    toolName: "WebSearch",
    timestamp: now,
  });
  const [page] = buildBrowserAgentPages({
    event: search,
    preview: search.result_preview,
    query: search.target,
    related_events: [search],
    target: search.target,
    web_url_builder: proxyUrl,
  });

  assert.equal(page.kind, "search");
  assert.equal(page.iframe_url, null);
  assert.equal(page.results[0].url, "https://example.com/reference");
});

test("SDK JSON envelopes become clean search results and Reader paragraphs", () => {
  const search = webEvent({
    id: "json-search",
    input: { query: "Nexus browser" },
    result: JSON.stringify({
      content: JSON.stringify({
        results: [{
          title: "Nexus Browser",
          url: "https://example.com/browser",
          snippet: "A real browser result from an SDK envelope.",
        }],
      }),
    }),
    target: "Nexus browser",
    toolName: "WebSearch",
    timestamp: now,
  });
  const fetch = webEvent({
    id: "json-fetch",
    input: { prompt: "Find the implementation", url: "https://example.com/browser" },
    result: JSON.stringify({
      content: "Prompt: Find the implementation\n# Browser implementation\nThe page keeps its navigation state.",
      error_code: null,
      is_error: false,
    }),
    target: "https://example.com/browser",
    toolName: "WebFetch",
    timestamp: now + 1,
  });
  const pages = buildBrowserAgentPages({
    event: fetch,
    preview: fetch.result_preview,
    query: fetch.target,
    related_events: [search, fetch],
    target: fetch.target,
  });

  assert.equal(pages[0].results[0].url, "https://example.com/browser");
  assert.deepEqual(
    pages[1].reader.paragraphs.map((paragraph) => paragraph.text),
    ["Browser implementation", "The page keeps its navigation state."],
  );
});

test("URL plus plain-text search output becomes one normal search result", () => {
  const search = webEvent({
    id: "line-search",
    input: { query: "macOS window design" },
    result: [
      "https://developer.apple.com/design/human-interface-guidelines/windows",
      "Windows provide a frame for focused content.",
      "Toolbars keep frequent actions close to the current task.",
    ],
    target: "macOS window design",
    toolName: "WebSearch",
    timestamp: now,
  });
  const [page] = buildBrowserAgentPages({
    event: search,
    preview: search.result_preview,
    query: search.target,
    related_events: [search],
    target: search.target,
  });

  assert.equal(page.results.length, 1);
  assert.equal(page.results[0].url, "https://developer.apple.com/design/human-interface-guidelines/windows");
  assert.match(page.results[0].snippet, /Toolbars keep frequent actions/);
  assert.doesNotMatch(page.results[0].title, /工具返回|摘要片段/);
});

test("explicit workspace HTML opens through the directory site URL", () => {
  const open = {
    ...webEvent({
      id: "open-html",
      input: { command: "open sites/gomoku/index.html" },
      result: { content: "", exit_code: 0, is_error: false },
      target: "sites/gomoku/index.html",
      toolName: "Bash",
      timestamp: now,
    }),
    kind: "command_run",
    surface: "terminal",
  };
  const pages = buildBrowserAgentPages({
    event: open,
    preview: open.result_preview,
    query: open.target,
    raw_url_builder: (agentId, filePath) => `/nexus/v1/agents/${agentId}/workspace/site/${filePath}`,
    related_events: [],
    target: open.target,
  });

  assert.equal(pages[0].kind, "workspace");
  assert.equal(pages[0].srcdoc, null);
  assert.equal(
    pages[0].iframe_url,
    "/nexus/v1/agents/agent:test/workspace/site/sites/gomoku/index.html",
  );

  const livePages = buildBrowserAgentPages({
    event: open,
    preview: "<!doctype html><html><head><title>Gomoku</title></head><body><script src='./game.js'></script></body></html>",
    query: open.target,
    raw_url_builder: (agentId, filePath) => `/nexus/v1/agents/${agentId}/workspace/site/${filePath}`,
    related_events: [],
    target: open.target,
  });
  assert.equal(livePages[0].kind, "workspace");
  assert.match(livePages[0].srcdoc, /<base href="\/nexus\/v1\/agents\/agent:test\/workspace\/site\/sites\/gomoku\/">/);
  assert.equal(livePages[0].iframe_url, pages[0].iframe_url);

  const reloaded = reloadBrowserPage(createBrowserNavigationState("workspace:html", livePages));
  assert.equal(activeBrowserPage(reloaded).srcdoc, null);
  assert.equal(activeBrowserPage(reloaded).iframe_url, pages[0].iframe_url);
});

test("one tool lifecycle updates one history entry and reader text removes tool envelope noise", () => {
  const search = webEvent({
    id: "dedupe-search",
    input: { query: "managed agents" },
    result: [{ title: "Managed agents", url: "https://example.com/agents", snippet: "Architecture" }],
    target: "managed agents",
    toolName: "WebSearch",
    timestamp: now,
  });
  const fetchRunning = webEvent({
    id: "fetch-running",
    input: { prompt: "managed agent architecture", url: "https://example.com/agents" },
    phase: "running",
    result: null,
    target: "https://example.com/agents",
    toolName: "WebFetch",
    timestamp: now + 1,
  });
  const fetchDone = {
    ...fetchRunning,
    id: "fetch-done",
    phase: "done",
    result_preview: {
      content: "Prompt: managed agent architecture\n[Skip to main content](#main)\n![](https://example.com/hero.png)\n- Products\n# Managed agent architecture\nManaged agent architecture separates durable sessions from workers.",
      is_error: false,
    },
    updated_at: now + 2,
  };
  fetchRunning.tool_use_id = "tool:fetch-lifecycle";
  fetchDone.tool_use_id = "tool:fetch-lifecycle";

  const pages = buildBrowserAgentPages({
    event: fetchDone,
    preview: fetchDone.result_preview,
    query: "https://example.com/agents",
    related_events: [search, fetchRunning, fetchDone],
    target: fetchDone.target,
  });

  assert.equal(pages.length, 2);
  assert.equal(pages[1].id, "agent:tool:fetch-lifecycle");
  assert.equal(pages[1].event.phase, "done");
  assert.deepEqual(
    pages[1].reader.paragraphs.map((paragraph) => paragraph.text),
    [
      "Managed agent architecture",
      "Managed agent architecture separates durable sessions from workers.",
    ],
  );
  assert.equal(pages[1].reader.highlighted_count, 2);
});

test("a new agent tool page takes focus without erasing user navigation", () => {
  const { pages } = buildToolPages();
  let state = createBrowserNavigationState("session:round", [pages[0]]);
  state = navigateBrowserAddress(state, "https://example.org/manual", proxyUrl);
  assert.equal(activeBrowserPage(state).address, "https://example.org/manual");

  state = syncBrowserAgentPages(state, "session:round", pages);
  assert.equal(state.active_tab_id, "agent");
  assert.equal(activeBrowserPage(state).id, "agent:tool:fetch");
  assert.ok(activeBrowserTab(state).pages.some((page) => page.address === "https://example.org/manual"));

  state = moveBrowserHistory(state, -1);
  assert.equal(activeBrowserPage(state).address, "https://example.org/manual");
});

test("completed sessions keep functional tabs, history, reload, and reader mode", () => {
  const { pages } = buildToolPages();
  let state = createBrowserNavigationState("session:round", pages);
  state = openBrowserTab(state);
  const userTabId = state.active_tab_id;
  state = navigateBrowserAddress(state, "example.com/one", proxyUrl);
  state = navigateBrowserAddress(state, "https://example.com/two", proxyUrl);
  assert.equal(activeBrowserPage(state).address, "https://example.com/two");

  state = moveBrowserHistory(state, -1);
  assert.equal(activeBrowserPage(state).address, "https://example.com/one");
  state = reloadBrowserPage(state);
  assert.equal(activeBrowserPage(state).reload_key, 1);
  assert.equal(state.active_tab_id, userTabId);

  state = navigateBrowserAddress(state, "https://example.com/stage", proxyUrl);
  assert.ok(activeBrowserPage(state).reader);
  state = toggleBrowserReader(state);
  assert.equal(activeBrowserPage(state).presentation, "reader");
});

test("user-entered domains keep real addresses while iframe loading stays inside Navi", () => {
  const urlPage = createBrowserUserPage({
    id: "url",
    input: "example.com/docs",
    web_url_builder: proxyUrl,
  });
  const searchPage = createBrowserUserPage({ id: "search", input: "unseen query" });

  assert.equal(urlPage.address, "https://example.com/docs");
  assert.equal(urlPage.iframe_url, proxyUrl("https://example.com/docs"));
  assert.equal(searchPage.kind, "search");
  assert.equal(searchPage.results.length, 0);
  assert.equal(searchPage.status.label, "本轮无搜索记录");
});

test("sandbox page messages only accept valid Navi navigation and error events", () => {
  assert.deepEqual(
    readBrowserPageBridgeMessage({
      source: "nexus-navi-proxy",
      type: "navigate",
      url: "https://example.com/next",
    }),
    { type: "navigate", url: "https://example.com/next" },
  );
  assert.deepEqual(
    readBrowserPageBridgeMessage({ source: "nexus-navi-proxy", type: "load-error", status: 502 }),
    { status: 502, type: "load-error" },
  );
  assert.equal(
    readBrowserPageBridgeMessage({ source: "other", type: "navigate", url: "https://example.com" }),
    null,
  );
  assert.equal(
    readBrowserPageBridgeMessage({ source: "nexus-navi-proxy", type: "navigate", url: "javascript:alert(1)" }),
    null,
  );
});

test("browser surface renders real controls and clickable search results", () => {
  const search = webEvent({
    id: "surface-search",
    input: { query: "Navi browser" },
    result: [{
      title: "Navi browser result",
      url: "https://example.com/navi",
      snippet: "Open this result inside the stage browser.",
    }],
    target: "Navi browser",
    toolName: "WebSearch",
    timestamp: now,
  });
  const markup = renderToStaticMarkup(createElement(BrowserSurface, {
    event: search,
    preview: search.result_preview,
    query: "Navi browser",
    relatedEvents: [search],
    target: search.target,
  }));

  assert.match(markup, /aria-label="后退"/);
  assert.match(markup, /aria-label="前进"/);
  assert.match(markup, /aria-label="重新载入"/);
  assert.match(markup, /aria-label="新建标签页"/);
  assert.match(markup, /Navi browser result/);
  assert.match(markup, /example\.com\/navi/);
  assert.doesNotMatch(markup, /在浏览器中打开/);
});
