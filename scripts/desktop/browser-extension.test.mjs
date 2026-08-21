import assert from "node:assert/strict";
import { webcrypto } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const event = { addListener() {}, removeListener() {} };
const actionOrder = [];
const cursorMessages = [];
const detachedTabs = [];
const removedTabs = [];
const ungroupedTabs = [];
let cursorReceiverAvailable = true;
const chrome = {
  action: {
    async setBadgeBackgroundColor() {},
    async setBadgeText() {},
  },
  alarms: { create() {}, onAlarm: event },
  contextMenus: { create() {}, onClicked: event, remove(_id, callback) { callback(); } },
  debugger: {
    onDetach: event,
    onEvent: event,
    async detach({ tabId }) { detachedTabs.push(tabId); },
  },
  runtime: { getManifest: () => ({ version: "test" }), onInstalled: event, onMessage: event },
  scripting: {
    async executeScript() {
      actionOrder.push("inject");
      cursorReceiverAvailable = true;
    },
  },
  storage: {
    local: {
      async get() { return { browser_enabled: false }; },
      async remove() {},
      async set() {},
    },
  },
  tabGroups: { onRemoved: event },
  tabs: {
    onActivated: event,
    onRemoved: event,
    onUpdated: event,
    async create() {},
    async get(tabId) {
      return { id: tabId, title: "Child", url: "https://example.com/child", groupId: -1, windowId: 1 };
    },
    async remove(tabId) { removedTabs.push(tabId); },
    async query() { return []; },
    async sendMessage(tabId, message) {
      actionOrder.push("cursor");
      cursorMessages.push({ tabId, message });
      if (!cursorReceiverAvailable) throw new Error("No receiving end");
      return { ok: true };
    },
    async ungroup(tabId) { ungroupedTabs.push(tabId); },
  },
  webNavigation: { onCreatedNavigationTarget: event },
};
class WebSocketStub {}
WebSocketStub.OPEN = 1;

const testPath = fileURLToPath(import.meta.url);
const backgroundPath = new URL("../../desktop/browser-extension/background.js", import.meta.url);
const source = await readFile(backgroundPath, "utf8");
const cursorPath = new URL("../../desktop/browser-extension/cursor.js", import.meta.url);
const cursorSource = await readFile(cursorPath, "utf8");
const context = vm.createContext({
  chrome,
  clearTimeout,
  console,
  crypto: webcrypto,
  setTimeout,
  TextEncoder,
  URL,
  WebSocket: WebSocketStub,
});
vm.runInContext(
  source + "\n;globalThis.__test = { BrowserController, SNAPSHOT_MAX_BYTES, SNAPSHOT_MAX_NODES, browserNameFromUserAgent, buildNexusContextPrompt, buildNexusLaunchURL, isControllableURL };",
  context,
  { filename: testPath },
);

test("Browser 能识别 Chrome 与 Edge", () => {
  const { browserNameFromUserAgent } = context.__test;
  assert.equal(browserNameFromUserAgent("Mozilla/5.0 Chrome/151.0 Safari/537.36"), "Google Chrome");
  assert.equal(browserNameFromUserAgent("Mozilla/5.0 Chrome/151.0 Edg/151.0"), "Microsoft Edge");
});

test("Browser 右键入口只交接可控网页上下文", () => {
  const { buildNexusContextPrompt, buildNexusLaunchURL, isControllableURL } = context.__test;
  const prompt = buildNexusContextPrompt(
    { pageUrl: "https://example.com/post", selectionText: "一段选中文本" },
    { title: "Example", url: "https://example.com/post" },
  );
  assert.match(prompt, /一段选中文本/);
  assert.match(prompt, /https:\/\/example\.com\/post/);
  assert.equal(new URL(buildNexusLaunchURL(prompt)).searchParams.get("initial"), prompt);
  assert.equal(isControllableURL("https://example.com"), true);
  assert.equal(isControllableURL("chrome://settings"), false);
});

test("Browser 快照有界且 evaluate 等待 Promise", async () => {
  const { BrowserController, SNAPSHOT_MAX_BYTES, SNAPSHOT_MAX_NODES } = context.__test;
  const controller = new BrowserController();
  const childIds = Array.from({ length: 450 }, (_, index) => String(index + 2));
  childIds.push("button");
  const nodes = [
    { nodeId: "root", childIds, role: { value: "RootWebArea" }, name: { value: "Example" } },
    ...childIds.slice(0, -1).map((nodeId, index) => ({
      nodeId,
      role: { value: "StaticText" },
      name: { value: "内容 " + index + " " + "中".repeat(300) },
    })),
    { nodeId: "button", backendDOMNodeId: 999, role: { value: "button" }, name: { value: "发布" } },
  ];

  const snapshot = controller.buildAccessibilitySnapshot(9, nodes);
  assert.equal(snapshot.truncated, true);
  assert.ok(snapshot.nodeCount < SNAPSHOT_MAX_NODES);
  assert.ok(new TextEncoder().encode(snapshot.snapshot).length <= SNAPSHOT_MAX_BYTES);
  assert.match(snapshot.snapshot, /@e1 button "发布"/);
  assert.equal(controller.refsByTab.get(9).get("@e1"), 999);

  const firstRevision = controller.buildSnapshotRevision(9, snapshot.snapshot, false);
  const unchanged = controller.buildAccessibilitySnapshot(9, nodes);
  const secondRevision = controller.buildSnapshotRevision(9, unchanged.snapshot, false);
  assert.equal(firstRevision.snapshot_type, "full");
  assert.equal(secondRevision.snapshot_type, "unchanged");
  assert.match(unchanged.snapshot, /@e1 button "发布"/);

  nodes.at(-1).name.value = "确认发布";
  const changed = controller.buildAccessibilitySnapshot(9, nodes);
  const thirdRevision = controller.buildSnapshotRevision(9, changed.snapshot, false);
  assert.equal(thirdRevision.snapshot_type, "diff");
  assert.match(thirdRevision.snapshot, /- .*@e1 button "发布"/);
  assert.match(thirdRevision.snapshot, /\+ .*@e1 button "确认发布"/);
  assert.equal(controller.buildSnapshotRevision(9, changed.snapshot, true).snapshot_type, "full");

  let evaluateParams;
  controller.getTab = async () => ({ id: 9 });
  controller.command = async (_tabId, _method, params) => {
    evaluateParams = params;
    return { result: { type: "string", value: "done" } };
  };
  assert.equal((await controller.evaluate({ tab_id: 9, code: "Promise.resolve('done')" })).value, "done");
  assert.equal(evaluateParams.awaitPromise, true);
  assert.equal(evaluateParams.timeout, 80000);
});

test("Browser 标签页引用绑定扩展代次和标签页实例", () => {
  const { BrowserController } = context.__test;
  const controller = new BrowserController();
  controller.setIdentity("browser-a", "generation-a");

  const ref = controller.tabRef(9);
  assert.equal(controller.parseTabRef(ref), 9);
  controller.clearTab(9);
  assert.throws(() => controller.parseTabRef(ref), /Stale tab_ref/);

  controller.setIdentity("browser-a", "generation-b");
  assert.throws(() => controller.parseTabRef(ref), /Stale tab_ref/);
});

test("Browser 新建导航目标继承来源标签页的 Session", async () => {
  const { BrowserController } = context.__test;
  const controller = new BrowserController();
  controller.setIdentity("browser-a", "generation-a");
  controller.claimTab(9, { session: "session-a", group_title: "Agent A", round_id: "round-a" }, false);
  const sourceRef = controller.tabRef(9);
  const events = [];
  controller.setEventSink((eventName, data) => events.push({ eventName, data }));
  controller.groupTab = async () => {};

  await controller.inheritCreatedTab({ sourceTabId: 9, tabId: 10 });

  assert.equal(JSON.stringify(controller.sessionTabIDs({ session: "session-a", tab_refs: [sourceRef] })), "[9,10]");
  assert.equal(events.length, 1);
  assert.equal(events[0].eventName, "tab_created");
  assert.equal(events[0].data.session, "session-a");
  assert.equal(events[0].data.source_tab_ref, sourceRef);
  assert.equal(events[0].data.tab.tab_id, 10);
  assert.equal(controller.parseTabRef(events[0].data.tab.tab_ref), 10);
  assert.equal(controller.leaseByTab.get(10).owned, true);
  assert.equal(controller.leaseByTab.get(10).roundID, "round-a");
});

test("Browser round 收尾关闭临时页并释放用户页", async () => {
  const { BrowserController } = context.__test;
  const controller = new BrowserController();
  controller.setIdentity("browser-a", "generation-a");
  detachedTabs.length = 0;
  removedTabs.length = 0;
  ungroupedTabs.length = 0;
  cursorMessages.length = 0;
  cursorReceiverAvailable = true;
  for (const [tabId, owned, mark] of [
    [1, true, ""],
    [2, false, ""],
    [3, true, "deliverable"],
    [4, true, "handoff"],
    [6, false, "deliverable"],
  ]) {
    controller.claimTab(tabId, { session: "session-a", round_id: "round-a" }, owned);
    controller.leaseByTab.get(tabId).mark = mark;
    controller.attachedTabs.add(tabId);
  }
  controller.claimTab(5, { session: "session-a", round_id: "round-b" }, true);

  const result = await controller.execute("finalize_round", {
    session: "session-a",
    round_id: "round-a",
  });

  assert.equal(JSON.stringify(result), JSON.stringify({ closed: 1, released: 3, handoff: 1 }));
  assert.deepEqual(removedTabs, [1]);
  assert.deepEqual(detachedTabs, [2, 3, 6]);
  assert.deepEqual(ungroupedTabs, [3]);
  assert.deepEqual(
    cursorMessages.map(({ tabId, message }) => [tabId, message.type]),
    [[2, "NEXUS_CURSOR_HIDE"], [3, "NEXUS_CURSOR_HIDE"], [4, "NEXUS_CURSOR_HIDE"], [6, "NEXUS_CURSOR_HIDE"]],
  );
  assert.equal(controller.leaseByTab.has(1), false);
  assert.equal(controller.leaseByTab.has(2), false);
  assert.equal(controller.leaseByTab.has(3), false);
  assert.equal(controller.leaseByTab.has(6), false);
  assert.equal(controller.leaseByTab.get(4).roundID, "");
  assert.equal(controller.leaseByTab.get(4).mark, "");
  assert.equal(controller.leaseByTab.get(5).roundID, "round-b");
});

test("Browser 标签页事件同步并在导航时废弃旧快照", async () => {
  const { BrowserController } = context.__test;
  const controller = new BrowserController();
  controller.setIdentity("browser-a", "generation-a");
  controller.claimTab(9, { session: "session-a", round_id: "round-a" }, false);
  controller.tabRef(9);
  controller.refsByTab.set(9, new Map([["@e1", 999]]));
  controller.snapshotByTab.set(9, { id: 1, lines: ["- @e1 button"] });
  const events = [];
  controller.setEventSink((eventName, data) => events.push({ eventName, data }));

  await controller.handleTabUpdated(9, { status: "loading", url: "https://example.com/next" }, {
    id: 9,
    title: "Next",
    url: "https://example.com/next",
    status: "loading",
    active: true,
    groupId: -1,
    windowId: 1,
  });
  await controller.handleTabActivated(9);
  controller.handleTabRemoved(9);

  assert.equal(controller.refsByTab.has(9), false);
  assert.equal(controller.snapshotByTab.has(9), false);
  assert.equal(events.map(({ eventName }) => eventName).join(","), "tab_updated,tab_activated,tab_removed");
});

test("Browser 标准点击等待可见光标抵达后再发送 CDP 事件", async () => {
  const { BrowserController } = context.__test;
  const controller = new BrowserController();
  actionOrder.length = 0;
  cursorMessages.length = 0;
  cursorReceiverAvailable = false;
  controller.getTab = async () => ({ id: 9 });
  controller.pointerPoint = async () => ({ x: 120, y: 80 });
  controller.command = async (_tabId, method, params) => {
    actionOrder.push(method + ":" + params.type);
    return {};
  };

  await controller.click({ tab_id: 9, selector: "@e1" });

  assert.deepEqual(actionOrder, [
    "cursor",
    "inject",
    "cursor",
    "Input.dispatchMouseEvent:mouseMoved",
    "Input.dispatchMouseEvent:mousePressed",
    "Input.dispatchMouseEvent:mouseReleased",
  ]);
  assert.equal(cursorMessages[1].tabId, 9);
  assert.equal(cursorMessages[1].message.type, "NEXUS_CURSOR_MOVE");
  assert.equal(cursorMessages[1].message.x, 120);
  assert.equal(cursorMessages[1].message.y, 80);
});

test("Browser 光标首个动作沿轨迹移动且只在当前可见标签页显示", async () => {
  let messageListener;
  let visibilityListener;
  const documentStub = {
    hidden: true,
    addEventListener(type, listener) {
      if (type === "visibilitychange") visibilityListener = listener;
    },
  };
  class MutationObserverStub {
    observe() {}
  }
  const cursorContext = vm.createContext({
    chrome: {
      runtime: {
        onMessage: {
          addListener(listener) { messageListener = listener; },
        },
      },
    },
    clearTimeout,
    document: documentStub,
    MutationObserver: MutationObserverStub,
    setTimeout,
    window: {
      innerHeight: 800,
      innerWidth: 1000,
      matchMedia: () => ({ matches: false }),
    },
  });
  vm.runInContext(
    cursorSource + "\n;globalThis.__cursorOverlay = cursorOverlay;",
    cursorContext,
    { filename: testPath },
  );

  const overlay = cursorContext.__cursorOverlay;
  overlay.cursor = { style: { opacity: "1" } };
  visibilityListener();
  assert.equal(overlay.cursor.style.opacity, "0");

  let response;
  assert.equal(messageListener(
    { type: "NEXUS_CURSOR_MOVE", x: 120, y: 80 },
    null,
    (value) => { response = value; },
  ), true);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(response.ok, true);
  assert.equal(overlay.host, null);

  let motionFrames;
  let motionOptions;
  let settleFrames;
  let settleOptions;
  documentStub.hidden = false;
  overlay.mount = () => {};
  overlay.cursor = {
    animate(value, options) {
      motionFrames = value;
      motionOptions = options;
      return { cancel() {}, finished: Promise.resolve() };
    },
    style: { opacity: "0" },
  };
  overlay.pointer = {
    animate(value, options) {
      settleFrames = value;
      settleOptions = options;
      return { cancel() {}, finished: Promise.resolve() };
    },
  };
  await overlay.move(120, 80);
  assert.ok(motionFrames.length >= 16);
  assert.equal(motionOptions.easing, "linear");
  assert.match(motionFrames[0].transform, /^translate3d\(577\.5px, 438\.2px, 0\)/);
  assert.match(motionFrames.at(-1).transform, /^translate3d\(117\.5px, 78\.2px, 0\)/);
  assert.ok(motionFrames.some(({ transform }) => /rotate\((?!0(?:\.0+)?deg)/.test(transform)));
  assert.equal(settleFrames.length, 48);
  assert.equal(settleOptions.duration, 1410);
  assert.equal(settleFrames[0].transform, "rotate(0deg)");
  assert.equal(settleFrames.at(-1).transform, "rotate(0deg)");
  assert.ok(settleFrames.some(({ transform }) => transform !== "rotate(0deg)"));
  assert.equal(overlay.cursor.style.opacity, "1");

  overlay.cursor.style.opacity = "1";
  assert.equal(messageListener(
    { type: "NEXUS_CURSOR_HIDE" },
    null,
    (value) => { response = value; },
  ), false);
  assert.equal(response.ok, true);
  assert.equal(overlay.cursor.style.opacity, "0");
  assert.equal(overlay.current, null);
});
