import assert from "node:assert/strict";
import { webcrypto } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const event = { addListener() {}, removeListener() {} };
const actionOrder = [];
const cursorMessages = [];
let cursorReceiverAvailable = true;
const chrome = {
  action: {
    async setBadgeBackgroundColor() {},
    async setBadgeText() {},
  },
  alarms: { create() {}, onAlarm: event },
  debugger: { onDetach: event, onEvent: event },
  runtime: { getManifest: () => ({ version: "test" }), onMessage: event },
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
    onRemoved: event,
    async get(tabId) {
      return { id: tabId, title: "Child", url: "https://example.com/child", groupId: -1, windowId: 1 };
    },
    async sendMessage(tabId, message) {
      actionOrder.push("cursor");
      cursorMessages.push({ tabId, message });
      if (!cursorReceiverAvailable) throw new Error("No receiving end");
      return { ok: true };
    },
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
  source + "\n;globalThis.__test = { BrowserController, SNAPSHOT_MAX_BYTES, SNAPSHOT_MAX_NODES };",
  context,
  { filename: testPath },
);

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
  controller.claimTab(9, { session: "session-a", group_title: "Agent A" }, false);
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
});

test("Browser 鼠标等待可见光标抵达后再发送 CDP 事件", async () => {
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

  await controller.mouseClick({ tab_id: 9 });

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

test("Browser 光标只在当前可见标签页显示", async () => {
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
});
