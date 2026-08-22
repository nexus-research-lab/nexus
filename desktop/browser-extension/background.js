const PROTOCOL_VERSION = "5";
const SUBPROTOCOL = "nexus.browser.v1";
const DEFAULT_ENDPOINTS = [
  "ws://127.0.0.1:34343/nexus/v1/internal/browser/ws",
  "ws://127.0.0.1:8010/nexus/v1/internal/browser/ws",
];
const RECONNECT_ALARM = "nexus-browser-reconnect";
const STORAGE_URL = "browser_url";
const STORAGE_ENABLED = "browser_enabled";
const STORAGE_INSTANCE_ID = "browser_instance_id";
const CONTEXT_MENU_ID = "ask-nexus";
const CONTEXT_TEXT_MAX_CHARS = 2000;
const BROWSER_GENERATION = crypto.randomUUID();
const INTERACTIVE_ROLES = new Set([
  "button",
  "link",
  "textbox",
  "checkbox",
  "radio",
  "combobox",
  "listbox",
  "menuitem",
  "menuitemcheckbox",
  "menuitemradio",
  "option",
  "searchbox",
  "slider",
  "spinbutton",
  "switch",
  "tab",
  "treeitem",
]);
const SNAPSHOT_CONTEXT_ROLES = new Set([
  "dialog",
  "document",
  "form",
  "heading",
  "main",
  "navigation",
  "region",
  "rootwebarea",
]);
const SNAPSHOT_MAX_NODES = 300;
const SNAPSHOT_MAX_BYTES = 12000;
const PAGE_CONTENT_DEFAULT_MAX_CHARS = 12000;
const SNAPSHOT_TEXT_MAX_CHARS = 240;
const CURSOR_MOVE_MESSAGE = "NEXUS_CURSOR_MOVE";
const CURSOR_HIDE_MESSAGE = "NEXUS_CURSOR_HIDE";
const CURSOR_ARRIVAL_TIMEOUT_MS = 1500;
const GROUP_COLORS = ["blue", "purple", "cyan", "green", "yellow", "orange", "red", "pink", "grey"];
const PAPER_FORMATS = {
  letter: [8.5, 11],
  legal: [8.5, 14],
  a4: [8.27, 11.69],
  a3: [11.69, 16.54],
  tabloid: [11, 17],
};

function browserNameFromUserAgent(userAgent) {
  return /\bEdg\//.test(String(userAgent || "")) ? "Microsoft Edge" : "Google Chrome";
}

function isControllableURL(rawURL) {
  return /^https?:\/\//i.test(String(rawURL || "").trim());
}

function buildNexusContextPrompt(info, tab) {
  const selection = String(info?.selectionText || "").trim().slice(0, CONTEXT_TEXT_MAX_CHARS);
  const targetURL = [info?.linkUrl, info?.srcUrl, info?.pageUrl, tab?.url]
    .map((value) => String(value || "").trim())
    .find(isControllableURL) || "";
  const title = String(tab?.title || "").trim();
  if (selection) {
    return `请结合这段网页内容帮我：\n\n${selection}${targetURL ? `\n\n来源：${targetURL}` : ""}`;
  }
  if (isControllableURL(info?.linkUrl)) return `请查看并处理这个链接：${targetURL}`;
  return `请查看并处理这个网页${title ? `「${title}」` : ""}${targetURL ? `：${targetURL}` : ""}`;
}

function buildNexusLaunchURL(prompt) {
  const url = new URL("nexus://launcher");
  url.searchParams.set("initial", String(prompt || "").trim());
  return url.href;
}

class BrowserController {
  constructor() {
    this.attachedTabs = new Set();
    this.refsByTab = new Map();
    this.refByBackendByTab = new Map();
    this.nextRefByTab = new Map();
    this.snapshotByTab = new Map();
    this.snapshotSequence = 0;
    this.networkTabs = new Set();
    this.networkRequests = new Map();
    this.consoleTabs = new Set();
    this.consoleEntries = new Map();
    this.dialogs = new Map();
    this.groupBySession = new Map();
    this.leaseByTab = new Map();
    this.tabTokens = new Map();
    this.browserInstanceID = "";
    this.browserGeneration = "";
    this.eventSink = () => {};
    this.platform = null;

    chrome.tabs.onRemoved.addListener((tabId) => this.handleTabRemoved(tabId));
    chrome.tabs.onUpdated.addListener((tabId, change, tab) => {
      void this.handleTabUpdated(tabId, change, tab);
    });
    chrome.tabs.onActivated.addListener(({ tabId }) => {
      void this.handleTabActivated(tabId);
    });
    chrome.debugger.onDetach.addListener((source) => {
      if (source.tabId !== undefined) {
        this.clearTab(source.tabId);
      }
    });
    chrome.debugger.onEvent.addListener((source, method, params) => {
      if (source.tabId !== undefined) {
        this.handleNetworkEvent(source.tabId, method, params);
        this.handleConsoleEvent(source.tabId, method, params);
        this.handleDialogEvent(source.tabId, method, params);
      }
    });
    chrome.tabGroups.onRemoved.addListener((group) => {
      for (const [session, groupId] of this.groupBySession) {
        if (groupId === group.id) this.groupBySession.delete(session);
      }
    });
    chrome.webNavigation.onCreatedNavigationTarget.addListener((details) => {
      void this.inheritCreatedTab(details);
    });
  }

  setIdentity(instanceID, generation) {
    this.browserInstanceID = this.requireText(instanceID, "browser instance id is required");
    this.browserGeneration = this.requireText(generation, "browser generation is required");
  }

  setEventSink(sink) {
    this.eventSink = typeof sink === "function" ? sink : () => {};
  }

  async execute(action, params = {}) {
    params = { ...params };
    if (params.tab_ref !== undefined) {
      const tabId = this.parseTabRef(params.tab_ref);
      if (params.tab_id !== undefined && params.tab_id !== tabId) {
        throw new Error("tab_ref does not match tab_id");
      }
      params.tab_id = tabId;
    }
    this.touchLease(params);
    switch (action) {
      case "list_tabs":
        return this.listTabs(params);
      case "attach_active":
        return this.attachActive(params);
      case "attach_tab":
        return this.attachTab(params);
      case "mark_tab":
        return this.markTab(params);
      case "navigate":
        return this.navigate(params);
      case "find_tab":
        return this.findTab(params);
      case "back":
      case "forward":
      case "reload":
        return this.navigateHistory(action, params);
      case "history":
        return this.history(params);
      case "evaluate":
        return this.evaluate(params);
      case "page_content":
        return this.pageContent(params);
      case "wait_for":
        return this.waitForElement(params);
      case "wait_for_url":
        return this.waitForURL(params);
      case "network":
        return this.network(params);
      case "console":
        return this.console(params);
      case "dialog":
        return this.dialog(params);
      case "snapshot":
        return this.snapshot(params);
      case "click":
        return this.click(params);
      case "fill":
        return this.fill(params);
      case "check":
        return this.setChecked(params, true);
      case "uncheck":
        return this.setChecked(params, false);
      case "select_option":
        return this.selectOption(params);
      case "mouse_click":
        return this.mouseClick(params);
      case "double_click":
        return this.mouseClick({ ...params, click_count: 2 });
      case "hover":
      case "mouse_move":
        return this.mouseMove(params);
      case "drag":
        return this.drag(params);
      case "scroll":
        return this.scroll(params);
      case "cdp":
        return this.rawCDP(params);
      case "clipboard":
        return this.clipboard(params);
      case "key_type":
        return this.keyType(params);
      case "send_keys":
        return this.sendKeys(params);
      case "press_key":
        return this.sendKeys({ ...params, keys: params.keys || params.key });
      case "screenshot":
        return this.screenshot(params);
      case "save_as_pdf":
        return this.saveAsPDF(params);
      case "upload":
        return this.upload(params);
      case "download":
        return this.download(params);
      case "downloads":
        return this.downloads(params);
      case "close_tab":
      case "close":
        return this.closeTab(params);
      case "close_session":
        return this.closeSession(params);
      case "finalize_round":
        return this.finalizeRound(params);
      default:
        throw new Error("Unsupported browser action: " + action);
    }
  }

  async navigate(params) {
    const url = this.requireText(params.url, "navigate requires url");
    let tab = null;
    let created = false;
    if (!params.new_tab && Number.isInteger(params.tab_id)) {
      try {
        tab = await chrome.tabs.get(params.tab_id);
      } catch {
        tab = null;
      }
    }

    if (tab) {
      const currentURL = tab.url || tab.pendingUrl || "";
      const loading = this.waitForNextLoad(tab.id);
      if (currentURL === url) {
        await chrome.tabs.reload(tab.id);
      } else {
        await chrome.tabs.update(tab.id, { url });
      }
      await loading;
    } else {
      tab = await chrome.tabs.create({ url, active: false });
      created = true;
      await this.groupTab(tab.id, params.session, params.group_title);
      await this.waitForLoad(tab.id);
    }

    tab = await chrome.tabs.get(tab.id);
    await this.attachDebugger(tab.id);
    this.claimTab(tab.id, params, created);
    return { ...await this.tabResult(tab), created };
  }

  async findTab(params) {
    const pattern = this.requireText(params.url, "find_tab requires url");
    let tab = null;
    let borrowed = false;

    if (params.active) {
      const [active] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
      if (active && this.matchesURL(active.url || active.pendingUrl || "", pattern)) {
        tab = active;
        borrowed = true;
      }
    }
    if (!tab) {
      for (const tabId of this.sessionTabIDs(params)) {
        try {
          const candidate = await chrome.tabs.get(tabId);
          if (this.matchesURL(candidate.url || candidate.pendingUrl || "", pattern)) {
            tab = candidate;
            break;
          }
        } catch {
          // 已关闭的会话标签页由宿主在下一次操作时自然淘汰。
        }
      }
    }
    if (!tab?.id) {
      throw new Error("No matching tab found");
    }
    await this.attachDebugger(tab.id);
    this.claimTab(tab.id, params, false);
    return { ...await this.tabResult(tab), borrowed };
  }

  async listTabs(params) {
    if (params.scope === "all") {
      const tabs = await chrome.tabs.query({});
      return { scope: "all", tabs: await Promise.all(tabs.map((tab) => this.tabResult(tab))) };
    }
    const tabs = [];
    for (const tabId of this.sessionTabIDs(params)) {
      try {
        tabs.push(await this.tabResult(await chrome.tabs.get(tabId)));
      } catch {
        // 标签页可能在宿主发出请求前已由用户关闭。
      }
    }
    return { scope: "session", tabs };
  }

  async attachActive(params) {
    const [tab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
    if (!tab?.id) throw new Error("No active browser tab found");
    await this.attachDebugger(tab.id);
    this.claimTab(tab.id, params, false);
    return { ...await this.tabResult(tab), borrowed: true };
  }

  async attachTab(params) {
    const tab = await this.getTab(params.tab_id);
    await this.attachDebugger(tab.id);
    this.claimTab(tab.id, params, false);
    return { ...await this.tabResult(tab), borrowed: true };
  }

  async markTab(params) {
    const tab = await this.getTab(params.tab_id);
    const lease = this.leaseByTab.get(tab.id);
    const session = this.requireText(params.session, "mark_tab requires session");
    if (!lease || lease.session !== session) throw new Error("Tab does not belong to this Session");
    const mark = String(params.mark || "").toLowerCase();
    if (!["none", "deliverable", "handoff"].includes(mark)) {
      throw new Error("mark_tab mark must be none, deliverable, or handoff");
    }
    lease.roundID = this.requireText(params.round_id, "mark_tab requires round_id");
    lease.mark = mark === "none" ? "" : mark;
    this.leaseByTab.set(tab.id, lease);
    return { ...await this.tabResult(tab), marked: mark };
  }

  async navigateHistory(action, params) {
    const tab = await this.getTab(params.tab_id);
    const loading = this.waitForNextLoad(tab.id);
    try {
      if (action === "back") await chrome.tabs.goBack(tab.id);
      else if (action === "forward") await chrome.tabs.goForward(tab.id);
      else await chrome.tabs.reload(tab.id, { bypassCache: false });
      await loading;
    } catch (error) {
      void loading.catch(() => {});
      throw error;
    }
    return { action, ...await this.tabResult(await chrome.tabs.get(tab.id)) };
  }

  async history(params) {
    const query = {
      text: String(params.query || ""),
      maxResults: Number.isInteger(params.max_results) ? params.max_results : 100,
    };
    if (Number.isFinite(params.start_time)) query.startTime = params.start_time;
    if (Number.isFinite(params.end_time)) query.endTime = params.end_time;
    const items = await chrome.history.search(query);
    return {
      count: items.length,
      items: items.map((item) => ({
        id: item.id,
        url: item.url || "",
        title: item.title || "",
        last_visit_time: item.lastVisitTime,
        visit_count: item.visitCount,
        typed_count: item.typedCount,
      })),
    };
  }

  async evaluate(params) {
    const tab = await this.getTab(params.tab_id);
    const response = await this.command(tab.id, "Runtime.evaluate", {
      expression: this.requireText(params.code, "evaluate requires code"),
      returnByValue: true,
      awaitPromise: true,
      timeout: Number.isInteger(params.timeout_ms) ? params.timeout_ms : 80000,
      userGesture: true,
    });
    this.assertRuntimeResult(response);
    return {
      tab_id: tab.id,
      value: response.result?.value ?? null,
      type: response.result?.type || "undefined",
      description: response.result?.description || "",
      unserializable_value: response.result?.unserializableValue,
    };
  }

  async pageContent(params) {
    const tab = await this.getTab(params.tab_id);
    const selector = String(params.selector || "").trim();
    const format = String(params.page_format || "text").toLowerCase();
    const maxChars = Number.isInteger(params.max_chars) ? params.max_chars : PAGE_CONTENT_DEFAULT_MAX_CHARS;
    let objectId;
    if (selector) {
      objectId = await this.resolveElement(tab.id, selector);
    } else {
      const root = await this.command(tab.id, "Runtime.evaluate", {
        expression: "document.documentElement",
        returnByValue: false,
      });
      this.assertRuntimeResult(root);
      objectId = root.result?.objectId;
    }
    if (!objectId) throw new Error("Unable to resolve page content root");
    try {
      const response = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: [
          "function(nextFormat, limit) {",
          "  const content = nextFormat === 'html' ? this.outerHTML : (this.innerText || this.textContent || '');",
          "  return { content: content.slice(0, limit), length: content.length, truncated: content.length > limit };",
          "}",
        ].join("\n"),
        arguments: [{ value: format }, { value: maxChars }],
        returnByValue: true,
      });
      this.assertRuntimeResult(response);
      return {
        tab_id: tab.id,
        title: tab.title || "",
        url: tab.url || "",
        format,
        ...(response.result?.value || { content: "", length: 0, truncated: false }),
      };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
  }

  async waitForElement(params) {
    const tab = await this.getTab(params.tab_id);
    const selector = this.requireText(params.selector, "wait_for requires selector");
    const state = String(params.state || "visible").toLowerCase();
    const expectedText = params.text === undefined ? null : String(params.text);
    const timeout = Number.isInteger(params.timeout_ms) ? params.timeout_ms : 30000;
    const deadline = Date.now() + timeout;
    while (Date.now() <= deadline) {
      const current = await this.elementState(tab.id, selector);
      const textMatches = expectedText === null || String(current.text).includes(expectedText);
      const matched = textMatches && (
        (state === "attached" && current.attached) ||
        (state === "detached" && !current.attached) ||
        (state === "visible" && current.visible) ||
        (state === "hidden" && (!current.attached || !current.visible))
      );
      if (matched) return { tab_id: tab.id, selector, state, matched: true, ...current };
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(`Timed out waiting for ${selector} to become ${state}`);
  }

  async waitForURL(params) {
    const tabId = this.parseTabID(params.tab_id);
    const pattern = this.requireText(params.url, "wait_for_url requires url");
    const timeout = Number.isInteger(params.timeout_ms) ? params.timeout_ms : 30000;
    const deadline = Date.now() + timeout;
    while (Date.now() <= deadline) {
      const tab = await chrome.tabs.get(tabId);
      const currentURL = tab.url || tab.pendingUrl || "";
      if (this.matchesURL(currentURL, pattern)) {
        return { tab_id: tabId, url: currentURL, title: tab.title || "", matched: true };
      }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(`Timed out waiting for URL ${pattern}`);
  }

  async elementState(tabId, selector) {
    const declaration = [
      "function() {",
      "  const style = getComputedStyle(this);",
      "  const rect = this.getBoundingClientRect();",
      "  const visible = style.visibility !== 'hidden' && style.display !== 'none' && rect.width > 0 && rect.height > 0;",
      "  return { attached: this.isConnected, visible, text: this.innerText || this.textContent || '' };",
      "}",
    ].join("\n");
    let objectId;
    try {
      objectId = await this.resolveElement(tabId, selector);
    } catch (error) {
      const message = String(error?.message || "");
      if (message.startsWith("No element matches") || message.startsWith("Unable to resolve ref")) {
        return { attached: false, visible: false, text: "" };
      }
      throw error;
    }
    try {
      const response = await this.command(tabId, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: declaration,
        returnByValue: true,
      });
      this.assertRuntimeResult(response);
      return response.result?.value || { attached: false, visible: false, text: "" };
    } finally {
      await this.releaseObject(tabId, objectId);
    }
  }

  async network(params) {
    const tab = await this.getTab(params.tab_id);
    const cmd = String(params.cmd || "").toLowerCase();
    if (cmd === "start") {
      await this.command(tab.id, "Network.enable");
      this.networkTabs.add(tab.id);
      this.networkRequests.set(tab.id, new Map());
      return { tab_id: tab.id, recording: true };
    }
    if (cmd === "stop") {
      if (this.networkTabs.has(tab.id)) {
        await this.command(tab.id, "Network.disable");
      }
      this.networkTabs.delete(tab.id);
      return {
        tab_id: tab.id,
        recording: false,
        count: this.networkRequests.get(tab.id)?.size || 0,
      };
    }
    if (cmd === "list") {
      const filter = String(params.filter || "").toLowerCase();
      const requests = [...(this.networkRequests.get(tab.id)?.values() || [])]
        .filter((item) => !filter || JSON.stringify(item).toLowerCase().includes(filter))
        .sort((left, right) => (left.timestamp || 0) - (right.timestamp || 0))
        .map((item) => ({
          request_id: item.request_id,
          method: item.method,
          url: item.url,
          status: item.status,
          status_text: item.status_text,
          resource_type: item.resource_type,
          mime_type: item.mime_type,
          encoded_data_length: item.encoded_data_length,
          finished: Boolean(item.finished),
          failed: Boolean(item.failed),
          error_text: item.error_text,
        }));
      return { tab_id: tab.id, recording: this.networkTabs.has(tab.id), count: requests.length, requests };
    }
    if (cmd === "detail") {
      const requestId = this.requireText(params.request_id, "network detail requires request_id");
      const detail = this.networkRequests.get(tab.id)?.get(requestId);
      if (!detail) throw new Error("Unknown network request: " + requestId);
      const response = await this.command(tab.id, "Network.getResponseBody", { requestId });
      let body = response.body || "";
      if (response.base64Encoded) {
        try {
          const bytes = Uint8Array.from(atob(body), (character) => character.charCodeAt(0));
          body = JSON.parse(new TextDecoder().decode(bytes));
        } catch {
          body = response.body || "";
        }
      } else {
        try {
          body = JSON.parse(body);
        } catch {
          // 非 JSON 响应保持原始文本。
        }
      }
      return { ...detail, body, base64_encoded: Boolean(response.base64Encoded) };
    }
    throw new Error("network cmd must be start, stop, list, or detail");
  }

  handleNetworkEvent(tabId, method, params) {
    if (!this.networkTabs.has(tabId)) return;
    const requests = this.networkRequests.get(tabId) || new Map();
    this.networkRequests.set(tabId, requests);
    const requestId = params.requestId;
    if (!requestId) return;
    const current = requests.get(requestId) || { request_id: requestId };

    if (method === "Network.requestWillBeSent") {
      requests.set(requestId, {
        ...current,
        request_id: requestId,
        url: params.request?.url || "",
        method: params.request?.method || "",
        request_headers: params.request?.headers || {},
        post_data: params.request?.postData,
        resource_type: params.type || "",
        timestamp: params.timestamp,
        wall_time: params.wallTime,
        redirect_response: params.redirectResponse,
      });
    } else if (method === "Network.responseReceived") {
      requests.set(requestId, {
        ...current,
        url: current.url || params.response?.url || "",
        status: params.response?.status,
        status_text: params.response?.statusText || "",
        response_headers: params.response?.headers || {},
        mime_type: params.response?.mimeType || "",
        protocol: params.response?.protocol || "",
        remote_ip: params.response?.remoteIPAddress || "",
        from_cache: Boolean(params.response?.fromDiskCache || params.response?.fromPrefetchCache),
        resource_type: params.type || current.resource_type,
      });
    } else if (method === "Network.loadingFinished") {
      requests.set(requestId, {
        ...current,
        encoded_data_length: params.encodedDataLength,
        finished: true,
      });
    } else if (method === "Network.loadingFailed") {
      requests.set(requestId, {
        ...current,
        failed: true,
        finished: true,
        canceled: Boolean(params.canceled),
        blocked_reason: params.blockedReason,
        error_text: params.errorText || "",
      });
    }
  }

  async console(params) {
    const tab = await this.getTab(params.tab_id);
    const cmd = String(params.cmd || "").toLowerCase();
    if (cmd === "start") {
      await this.command(tab.id, "Runtime.enable");
      await this.command(tab.id, "Log.enable");
      this.consoleTabs.add(tab.id);
      this.consoleEntries.set(tab.id, []);
      return { tab_id: tab.id, recording: true };
    }
    if (cmd === "stop") {
      this.consoleTabs.delete(tab.id);
      await Promise.allSettled([
        this.command(tab.id, "Runtime.disable"),
        this.command(tab.id, "Log.disable"),
      ]);
      return { tab_id: tab.id, recording: false, count: this.consoleEntries.get(tab.id)?.length || 0 };
    }
    if (cmd === "list") {
      const filter = String(params.filter || "").toLowerCase();
      const limit = Number.isInteger(params.max_results) ? params.max_results : 100;
      const entries = (this.consoleEntries.get(tab.id) || [])
        .filter((entry) => !filter || JSON.stringify(entry).toLowerCase().includes(filter))
        .slice(-limit);
      return { tab_id: tab.id, recording: this.consoleTabs.has(tab.id), count: entries.length, entries };
    }
    throw new Error("console cmd must be start, stop, or list");
  }

  handleConsoleEvent(tabId, method, params) {
    if (!this.consoleTabs.has(tabId)) return;
    const entries = this.consoleEntries.get(tabId) || [];
    if (method === "Runtime.consoleAPICalled") {
      entries.push({
        source: "console",
        level: params.type || "log",
        timestamp: params.timestamp,
        values: (params.args || []).map((value) => this.remoteValue(value)),
        stack_trace: params.stackTrace,
      });
    } else if (method === "Log.entryAdded") {
      const entry = params.entry || {};
      entries.push({
        source: entry.source || "log",
        level: entry.level || "info",
        text: entry.text || "",
        url: entry.url || "",
        line_number: entry.lineNumber,
        timestamp: entry.timestamp,
        stack_trace: entry.stackTrace,
      });
    } else {
      return;
    }
    if (entries.length > 1000) entries.splice(0, entries.length - 1000);
    this.consoleEntries.set(tabId, entries);
  }

  remoteValue(value) {
    if (Object.prototype.hasOwnProperty.call(value || {}, "value")) return value.value;
    return value?.unserializableValue || value?.description || value?.type || "undefined";
  }

  async dialog(params) {
    const tab = await this.getTab(params.tab_id);
    const cmd = String(params.cmd || "").toLowerCase();
    if (cmd === "get") {
      return { tab_id: tab.id, dialog: this.dialogs.get(tab.id) || null };
    }
    const current = this.dialogs.get(tab.id) || null;
    if (!current) return { tab_id: tab.id, handled: false, reason: "no open dialog" };
    const accept = cmd === "accept";
    const commandParams = { accept };
    if (accept && params.prompt_text !== undefined) commandParams.promptText = String(params.prompt_text);
    await this.command(tab.id, "Page.handleJavaScriptDialog", commandParams);
    this.dialogs.delete(tab.id);
    return { tab_id: tab.id, handled: true, accepted: accept, dialog: current };
  }

  handleDialogEvent(tabId, method, params) {
    if (method === "Page.javascriptDialogOpening") {
      this.dialogs.set(tabId, {
        type: params.type || "alert",
        message: params.message || "",
        default_prompt: params.defaultPrompt || "",
        url: params.url || "",
        has_browser_handler: Boolean(params.hasBrowserHandler),
      });
    } else if (method === "Page.javascriptDialogClosed") {
      this.dialogs.delete(tabId);
    }
  }

  async snapshot(params) {
    const tab = await this.getTab(params.tab_id);
    const response = await this.command(tab.id, "Accessibility.getFullAXTree");
    const rendered = this.buildAccessibilitySnapshot(tab.id, response.nodes || []);
    const revision = this.buildSnapshotRevision(tab.id, rendered.snapshot, Boolean(params.full));
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || "",
      ...revision,
      nodes: rendered.nodeCount,
      total_nodes: rendered.totalNodes,
      refs: rendered.refCount,
      truncated: rendered.truncated,
    };
  }

  buildAccessibilitySnapshot(tabId, nodes) {
    const byId = new Map(nodes.map((node) => [node.nodeId, node]));
    const childIds = new Set(nodes.flatMap((node) => node.childIds || []));
    const roots = nodes.filter((node) => !childIds.has(node.nodeId));
    const visited = new Set();
    const candidates = [];

    const visit = (nodeId, depth = 0) => {
      const node = byId.get(nodeId);
      if (!node || visited.has(nodeId)) return;
      visited.add(nodeId);
      const role = this.cleanText(node.role?.value).toLowerCase();
      const name = this.compactSnapshotText(node.name?.value);
      const value = this.compactSnapshotText(node.value?.value);
      const meaningful = !node.ignored && (name || value || (role && role !== "none" && role !== "generic"));
      if (meaningful) {
        const states = [];
        for (const key of ["disabled", "focused", "checked", "selected", "expanded", "level"]) {
          const property = node.properties?.find((candidate) => candidate.name === key)?.value?.value;
          if (property === true) states.push(key);
          else if (property !== undefined && property !== false) states.push(key + "=" + String(property));
          else if (property === false && ["checked", "selected", "expanded"].includes(key)) states.push(key + "=false");
        }
        const interactive = INTERACTIVE_ROLES.has(role) && node.backendDOMNodeId !== undefined;
        candidates.push({
          role: role || "unknown",
          name,
          value,
          description: this.compactSnapshotText(node.description?.value),
          states,
          interactive,
          backendDOMNodeId: node.backendDOMNodeId,
          depth,
          order: candidates.length,
          priority: interactive ? 0 : SNAPSHOT_CONTEXT_ROLES.has(role) ? 1 : name || value ? 2 : 3,
        });
      }
      const childDepth = depth + (meaningful ? 1 : 0);
      for (const childId of node.childIds || []) visit(childId, childDepth);
    };

    for (const root of roots.length ? roots : nodes.slice(0, 1)) visit(root.nodeId);

    let usedBytes = 0;
    const encoder = new TextEncoder();
    const selected = [];
    const prioritized = [...candidates].sort((left, right) => left.priority - right.priority || left.order - right.order);
    for (const candidate of prioritized) {
      if (selected.length >= SNAPSHOT_MAX_NODES) break;
      const estimate = encoder.encode(
        this.renderAccessibilityNode(candidate, candidate.interactive ? "@e000" : "") + "\n",
      ).length;
      if (usedBytes + estimate > SNAPSHOT_MAX_BYTES) continue;
      selected.push(candidate);
      usedBytes += estimate;
    }
    selected.sort((left, right) => left.order - right.order);

    const refs = new Map();
    const previousRefs = this.refByBackendByTab.get(tabId) || new Map();
    const currentRefs = new Map();
    let nextRef = this.nextRefByTab.get(tabId) || 1;
    const lines = selected.map((candidate) => {
      let ref = "";
      if (candidate.interactive) {
        ref = previousRefs.get(candidate.backendDOMNodeId) || "@e" + nextRef++;
        refs.set(ref, candidate.backendDOMNodeId);
        currentRefs.set(candidate.backendDOMNodeId, ref);
      }
      return this.renderAccessibilityNode(candidate, ref);
    });
    this.refsByTab.set(tabId, refs);
    this.refByBackendByTab.set(tabId, currentRefs);
    this.nextRefByTab.set(tabId, nextRef);
    return {
      snapshot: lines.join("\n"),
      nodeCount: selected.length,
      totalNodes: candidates.length,
      refCount: refs.size,
      truncated: selected.length < candidates.length,
    };
  }

  buildSnapshotRevision(tabId, snapshot, forceFull) {
    const lines = snapshot ? snapshot.split("\n") : [];
    const previous = this.snapshotByTab.get(tabId);
    const snapshotId = ++this.snapshotSequence;
    this.snapshotByTab.set(tabId, { id: snapshotId, lines });
    if (!previous || forceFull) {
      return { snapshot, snapshot_type: "full", snapshot_id: snapshotId };
    }

    const removed = this.lineDifference(previous.lines, lines).map((line) => "- " + line);
    const added = this.lineDifference(lines, previous.lines).map((line) => "+ " + line);
    const diff = [...removed, ...added].join("\n");
    if (!diff) {
      return {
        snapshot: "No accessibility changes.",
        snapshot_type: "unchanged",
        snapshot_id: snapshotId,
        base_snapshot_id: previous.id,
      };
    }
    if (new TextEncoder().encode(diff).length >= new TextEncoder().encode(snapshot).length) {
      return { snapshot, snapshot_type: "full", snapshot_id: snapshotId };
    }
    return {
      snapshot: diff,
      snapshot_type: "diff",
      snapshot_id: snapshotId,
      base_snapshot_id: previous.id,
    };
  }

  lineDifference(left, right) {
    const counts = new Map();
    for (const line of right) counts.set(line, (counts.get(line) || 0) + 1);
    return left.filter((line) => {
      const count = counts.get(line) || 0;
      if (!count) return true;
      counts.set(line, count - 1);
      return false;
    });
  }

  renderAccessibilityNode(node, ref) {
    const parts = ["  ".repeat(Math.min(node.depth, 8)) + "-", ref, node.role].filter(Boolean);
    if (node.name) parts.push(JSON.stringify(node.name));
    if (node.value && node.value !== node.name) parts.push("value=" + JSON.stringify(node.value));
    if (node.description && node.description !== node.name) {
      parts.push("description=" + JSON.stringify(node.description));
    }
    if (node.states.length) parts.push("[" + node.states.join(", ") + "]");
    return parts.join(" ");
  }

  async click(params) {
    return this.mouseClick({ ...params, button: "left", click_count: 1 });
  }

  async fill(params) {
    const tab = await this.getTab(params.tab_id);
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      const declaration = [
        "function(nextValue) {",
        "  this.focus();",
        "  if (this.isContentEditable) {",
        "    this.textContent = nextValue;",
        "    this.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: nextValue }));",
        "    this.dispatchEvent(new Event('change', { bubbles: true }));",
        "    return { tag: this.tagName, mode: 'contenteditable' };",
        "  }",
        "  const prototypes = { INPUT: HTMLInputElement.prototype, TEXTAREA: HTMLTextAreaElement.prototype, SELECT: HTMLSelectElement.prototype };",
        "  const prototype = prototypes[this.tagName];",
        "  if (!prototype) throw new Error('Element does not accept text input');",
        "  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set;",
        "  if (setter) setter.call(this, nextValue); else this.value = nextValue;",
        "  this.dispatchEvent(new Event('input', { bubbles: true }));",
        "  this.dispatchEvent(new Event('change', { bubbles: true }));",
        "  return { tag: this.tagName, mode: 'value' };",
        "}",
      ].join("\n");
      const response = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: declaration,
        arguments: [{ value: String(params.value ?? "") }],
        returnByValue: true,
        userGesture: true,
      });
      this.assertRuntimeResult(response);
      return { tab_id: tab.id, filled: true, selector: params.selector, ...(response.result?.value || {}) };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
  }

  async setChecked(params, checked) {
    const tab = await this.getTab(params.tab_id);
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      const response = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: [
          "function(nextChecked) {",
          "  if (!(this instanceof HTMLInputElement) || !['checkbox', 'radio'].includes(this.type)) {",
          "    throw new Error('Element must be a checkbox or radio input');",
          "  }",
          "  if (!nextChecked && this.type === 'radio') throw new Error('Radio inputs cannot be unchecked directly');",
          "  if (this.checked !== nextChecked) {",
          "    this.checked = nextChecked;",
          "    this.dispatchEvent(new Event('input', { bubbles: true }));",
          "    this.dispatchEvent(new Event('change', { bubbles: true }));",
          "  }",
          "  return { tag: this.tagName, type: this.type, checked: this.checked };",
          "}",
        ].join("\n"),
        arguments: [{ value: checked }],
        returnByValue: true,
        userGesture: true,
      });
      this.assertRuntimeResult(response);
      return { tab_id: tab.id, selector: params.selector, ...(response.result?.value || {}) };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
  }

  async selectOption(params) {
    const tab = await this.getTab(params.tab_id);
    const values = Array.isArray(params.values) ? params.values.map(String) : [String(params.value ?? "")];
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      const response = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: [
          "function(nextValues) {",
          "  if (!(this instanceof HTMLSelectElement)) throw new Error('Element must be a select');",
          "  const expected = new Set(nextValues);",
          "  for (const option of this.options) option.selected = expected.has(option.value) || expected.has(option.label);",
          "  this.dispatchEvent(new Event('input', { bubbles: true }));",
          "  this.dispatchEvent(new Event('change', { bubbles: true }));",
          "  return { values: Array.from(this.selectedOptions, option => option.value) };",
          "}",
        ].join("\n"),
        arguments: [{ value: values }],
        returnByValue: true,
        userGesture: true,
      });
      this.assertRuntimeResult(response);
      return { tab_id: tab.id, selector: params.selector, ...(response.result?.value || {}) };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
  }

  async mouseClick(params) {
    const tab = await this.getTab(params.tab_id);
    const point = await this.pointerPoint(tab.id, params, "selector", "x", "y");
    const button = String(params.button || "left").toLowerCase();
    const clickCount = Number.isInteger(params.click_count) ? params.click_count : 1;
    const buttons = { left: 1, right: 2, middle: 4, back: 8, forward: 16 }[button];
    await this.moveCursor(tab.id, point);
    await this.command(tab.id, "Input.dispatchMouseEvent", {
      type: "mouseMoved", x: point.x, y: point.y, button: "none",
    });
    for (let count = 1; count <= clickCount; count += 1) {
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mousePressed", x: point.x, y: point.y, button, buttons, clickCount: count,
      });
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mouseReleased", x: point.x, y: point.y, button, buttons: 0, clickCount: count,
      });
    }
    return { tab_id: tab.id, clicked: true, button, click_count: clickCount, ...point };
  }

  async mouseMove(params) {
    const tab = await this.getTab(params.tab_id);
    const point = await this.pointerPoint(tab.id, params, "selector", "x", "y");
    await this.moveCursor(tab.id, point);
    await this.command(tab.id, "Input.dispatchMouseEvent", {
      type: "mouseMoved", x: point.x, y: point.y, button: "none",
    });
    return { tab_id: tab.id, moved: true, ...point };
  }

  async drag(params) {
    const tab = await this.getTab(params.tab_id);
    const start = params.selector
      ? await this.pointerPoint(tab.id, params, "selector", "from_x", "from_y")
      : { x: Number(params.from_x), y: Number(params.from_y) };
    const target = params.target_selector
      ? await this.pointerPoint(tab.id, params, "target_selector", "to_x", "to_y")
      : { x: Number(params.to_x), y: Number(params.to_y) };
    const steps = Number.isInteger(params.steps) ? params.steps : 10;
    await this.moveCursor(tab.id, start);
    await this.command(tab.id, "Input.dispatchMouseEvent", {
      type: "mouseMoved", x: start.x, y: start.y, button: "none",
    });
    await this.command(tab.id, "Input.dispatchMouseEvent", {
      type: "mousePressed", x: start.x, y: start.y, button: "left", buttons: 1, clickCount: 1,
    });
    const cursorMove = this.moveCursor(tab.id, target);
    for (let step = 1; step <= steps; step += 1) {
      const ratio = step / steps;
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mouseMoved",
        x: start.x + (target.x - start.x) * ratio,
        y: start.y + (target.y - start.y) * ratio,
        button: "left",
        buttons: 1,
      });
    }
    await cursorMove;
    await this.command(tab.id, "Input.dispatchMouseEvent", {
      type: "mouseReleased", x: target.x, y: target.y, button: "left", buttons: 0, clickCount: 1,
    });
    return { tab_id: tab.id, dragged: true, from: start, to: target, steps };
  }

  async scroll(params) {
    const tab = await this.getTab(params.tab_id);
    let point;
    if (params.selector) {
      point = await this.pointerPoint(tab.id, params, "selector", "x", "y");
    } else if (Number.isFinite(params.x) && Number.isFinite(params.y)) {
      point = { x: params.x, y: params.y };
    } else {
      const metrics = await this.command(tab.id, "Page.getLayoutMetrics");
      const viewport = metrics.cssVisualViewport || metrics.visualViewport || {};
      point = { x: (viewport.clientWidth || 0) / 2, y: (viewport.clientHeight || 0) / 2 };
    }
    const deltaX = Number(params.delta_x || 0);
    const deltaY = Number(params.delta_y || 0);
    await this.moveCursor(tab.id, point);
    if (deltaX || deltaY) {
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mouseWheel", x: point.x, y: point.y, deltaX, deltaY,
      });
    }
    return { tab_id: tab.id, scrolled: true, x: point.x, y: point.y, delta_x: deltaX, delta_y: deltaY };
  }

  async moveCursor(tabId, point) {
    try {
      await this.sendCursorMove(tabId, point);
      return;
    } catch {
      try {
        await chrome.scripting.executeScript({
          files: ["cursor.js"],
          injectImmediately: true,
          target: { tabId },
        });
        await this.sendCursorMove(tabId, point);
      } catch {
        return;
      }
    }
  }

  sendCursorMove(tabId, point) {
    return new Promise((resolve, reject) => {
      let settled = false;
      const finish = (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        if (error) reject(error); else resolve();
      };
      const timeout = setTimeout(() => finish(new Error("Cursor arrival timed out")), CURSOR_ARRIVAL_TIMEOUT_MS);
      try {
        chrome.tabs.sendMessage(tabId, {
          type: CURSOR_MOVE_MESSAGE,
          x: point.x,
          y: point.y,
        }).then(() => finish(), finish);
      } catch (error) {
        finish(error);
      }
    });
  }

  async hideCursor(tabId) {
    try {
      await chrome.tabs.sendMessage(tabId, { type: CURSOR_HIDE_MESSAGE });
    } catch {
      // 未注入光标脚本的页面无需清理。
    }
  }

  async pointerPoint(tabId, params, selectorKey, xKey, yKey) {
    const selector = String(params[selectorKey] || "").trim();
    if (selector) return this.elementPoint(tabId, selector);
    const x = Number(params[xKey]);
    const y = Number(params[yKey]);
    if (!Number.isFinite(x) || !Number.isFinite(y)) throw new Error(`A valid ${xKey}/${yKey} point is required`);
    return { x, y };
  }

  async elementPoint(tabId, selector) {
    const objectId = await this.resolveElement(tabId, selector);
    try {
      await this.command(tabId, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: "function() { this.scrollIntoView({ block: 'center', inline: 'center' }); }",
      });
      const box = await this.command(tabId, "DOM.getBoxModel", { objectId });
      const quad = box.model?.border || box.model?.content;
      if (!Array.isArray(quad) || quad.length < 8) throw new Error("Element has no layout box");
      return {
        selector,
        x: (quad[0] + quad[2] + quad[4] + quad[6]) / 4,
        y: (quad[1] + quad[3] + quad[5] + quad[7]) / 4,
      };
    } finally {
      await this.releaseObject(tabId, objectId);
    }
  }

  async rawCDP(params) {
    const tab = await this.getTab(params.tab_id);
    const method = this.requireText(params.method, "cdp requires method");
    return {
      tab_id: tab.id,
      method,
      result: await this.command(tab.id, method, params.params || {}),
    };
  }

  async clipboard(params) {
    const tab = await this.getTab(params.tab_id);
    const cmd = String(params.cmd || "").toLowerCase();
    await this.ensureOffscreenDocument();
    const response = await chrome.runtime.sendMessage({
      target: "nexus-browser-offscreen",
      type: cmd === "write" ? "CLIPBOARD_WRITE" : "CLIPBOARD_READ",
      text: String(params.text ?? ""),
    });
    if (!response?.ok) throw new Error(response?.error || "Clipboard operation failed");
    return cmd === "write"
      ? { tab_id: tab.id, written: true, text_length: [...String(params.text ?? "")].length }
      : { tab_id: tab.id, text: response.text || "" };
  }

  async ensureOffscreenDocument() {
    const offscreenURL = chrome.runtime.getURL("offscreen.html");
    const contexts = await chrome.runtime.getContexts({
      contextTypes: ["OFFSCREEN_DOCUMENT"],
      documentUrls: [offscreenURL],
    });
    if (contexts.length) return;
    try {
      await chrome.offscreen.createDocument({
        url: "offscreen.html",
        reasons: ["CLIPBOARD"],
        justification: "Read and write the Browser session clipboard",
      });
    } catch (error) {
      if (!String(error?.message || "").includes("single offscreen document")) throw error;
    }
  }

  async keyType(params) {
    const tab = await this.getTab(params.tab_id);
    const text = String(params.text ?? "");
    await this.command(tab.id, "Input.insertText", { text });
    return { tab_id: tab.id, typed: true, text_length: [...text].length };
  }

  async sendKeys(params) {
    const tab = await this.getTab(params.tab_id);
    const keys = this.requireText(params.keys, "send_keys requires keys").split(/\s+/);
    const repeat = Number.isInteger(params.repeat) ? params.repeat : 1;
    if (repeat < 1 || repeat > 100) throw new Error("repeat must be between 1 and 100");
    for (let count = 0; count < repeat; count += 1) {
      for (const token of keys) {
        await this.dispatchKey(tab.id, await this.keySpec(token));
      }
    }
    return { tab_id: tab.id, sent: true, keys: params.keys, repeat };
  }

  async dispatchKey(tabId, spec) {
    await this.command(tabId, "Input.dispatchKeyEvent", {
      type: "keyDown",
      modifiers: spec.modifiers,
      key: spec.key,
      code: spec.code,
      windowsVirtualKeyCode: spec.keyCode,
      nativeVirtualKeyCode: spec.keyCode,
      ...(spec.text ? { text: spec.text } : {}),
    });
    await this.command(tabId, "Input.dispatchKeyEvent", {
      type: "keyUp",
      modifiers: spec.modifiers,
      key: spec.key,
      code: spec.code,
      windowsVirtualKeyCode: spec.keyCode,
      nativeVirtualKeyCode: spec.keyCode,
    });
  }

  async keySpec(rawToken) {
    const parts = String(rawToken || "").split("+").filter(Boolean);
    const rawKey = parts.pop();
    if (!rawKey) throw new Error("Empty key token");
    if (!this.platform) this.platform = (await chrome.runtime.getPlatformInfo()).os;

    let modifiers = 0;
    for (const modifier of parts) {
      switch (modifier.toLowerCase()) {
        case "alt":
          modifiers |= 1;
          break;
        case "ctrl":
        case "control":
          modifiers |= 2;
          break;
        case "cmd":
        case "command":
        case "meta":
          modifiers |= 4;
          break;
        case "shift":
          modifiers |= 8;
          break;
        case "mod":
          modifiers |= this.platform === "mac" ? 4 : 2;
          break;
        default:
          throw new Error("Unsupported modifier: " + modifier);
      }
    }

    const keyName = rawKey.toLowerCase();
    const special = {
      enter: ["Enter", "Enter", 13, "\r"],
      escape: ["Escape", "Escape", 27],
      esc: ["Escape", "Escape", 27],
      tab: ["Tab", "Tab", 9],
      backspace: ["Backspace", "Backspace", 8],
      delete: ["Delete", "Delete", 46],
      space: [" ", "Space", 32, " "],
      arrowup: ["ArrowUp", "ArrowUp", 38],
      arrowdown: ["ArrowDown", "ArrowDown", 40],
      arrowleft: ["ArrowLeft", "ArrowLeft", 37],
      arrowright: ["ArrowRight", "ArrowRight", 39],
      home: ["Home", "Home", 36],
      end: ["End", "End", 35],
      pageup: ["PageUp", "PageUp", 33],
      pagedown: ["PageDown", "PageDown", 34],
    };
    if (special[keyName]) {
      const [key, code, keyCode, rawText] = special[keyName];
      const text = modifiers & (1 | 2 | 4) ? "" : rawText;
      return { key, code, keyCode, text, modifiers };
    }
    if (/^f([1-9]|1[0-2])$/.test(keyName)) {
      const number = Number(keyName.slice(1));
      return { key: "F" + number, code: "F" + number, keyCode: 111 + number, modifiers };
    }
    if ([...rawKey].length === 1) {
      const lower = rawKey.toLowerCase();
      const upper = rawKey.toUpperCase();
      const shifted = Boolean(modifiers & 8);
      const key = shifted ? upper : lower;
      const isLetter = /[a-z]/i.test(rawKey);
      const isDigit = /[0-9]/.test(rawKey);
      const code = isLetter ? "Key" + upper : isDigit ? "Digit" + rawKey : "Unidentified";
      const keyCode = upper.charCodeAt(0);
      const text = modifiers & (1 | 2 | 4) ? "" : key;
      return { key, code, keyCode, text, modifiers };
    }
    throw new Error("Unsupported key: " + rawKey);
  }

  async screenshot(params) {
    const tab = await this.getTab(params.tab_id);
    const format = String(params.format || "png").toLowerCase();
    if (format !== "png" && format !== "jpeg") throw new Error("format must be png or jpeg");
    const commandParams = {
      format,
      fromSurface: true,
      captureBeyondViewport: Boolean(params.selector || params.full_page),
    };
    if (format === "jpeg" && Number.isInteger(params.quality)) commandParams.quality = params.quality;
    if (params.selector) {
      commandParams.clip = await this.elementClip(tab.id, params.selector);
    } else if (params.full_page) {
      const metrics = await this.command(tab.id, "Page.getLayoutMetrics");
      const size = metrics.cssContentSize || metrics.contentSize;
      if (!size?.width || !size?.height) throw new Error("Unable to measure full page");
      commandParams.clip = { x: 0, y: 0, width: size.width, height: size.height, scale: 1 };
    }
    const response = await this.command(tab.id, "Page.captureScreenshot", commandParams);
    if (!response.data) throw new Error("Chrome returned an empty screenshot");
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || "",
      mime_type: "image/" + format,
      full_page: Boolean(params.full_page),
      data: response.data,
    };
  }

  async elementClip(tabId, selector) {
    const objectId = await this.resolveElement(tabId, selector);
    try {
      await this.command(tabId, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: "function() { this.scrollIntoView({ block: 'center', inline: 'center' }); }",
      });
      const response = await this.command(tabId, "DOM.getBoxModel", { objectId });
      const quad = response.model?.border || response.model?.content;
      if (!Array.isArray(quad) || quad.length < 8) throw new Error("Element has no screenshot layout box");
      const xs = [quad[0], quad[2], quad[4], quad[6]];
      const ys = [quad[1], quad[3], quad[5], quad[7]];
      const x = Math.min(...xs);
      const y = Math.min(...ys);
      return {
        x,
        y,
        width: Math.max(...xs) - x,
        height: Math.max(...ys) - y,
        scale: 1,
      };
    } finally {
      await this.releaseObject(tabId, objectId);
    }
  }

  async saveAsPDF(params) {
    const tab = await this.getTab(params.tab_id);
    const format = String(params.paper_format || "a4").toLowerCase();
    const paper = PAPER_FORMATS[format];
    if (!paper) throw new Error("Unsupported paper format: " + format);
    const response = await this.command(tab.id, "Page.printToPDF", {
      paperWidth: paper[0],
      paperHeight: paper[1],
      scale: Number(params.scale || 1),
      landscape: Boolean(params.landscape),
      printBackground: Boolean(params.print_background),
      transferMode: "ReturnAsBase64",
    });
    if (!response.data) throw new Error("Chrome returned an empty PDF");
    const fallback = this.safeFileName(tab.title || "page") + ".pdf";
    const fileName = this.safeFileName(params.file_name || fallback, true);
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || "",
      file_name: fileName.toLowerCase().endsWith(".pdf") ? fileName : fileName + ".pdf",
      mime_type: "application/pdf",
      data: response.data,
    };
  }

  async upload(params) {
    const tab = await this.getTab(params.tab_id);
    const files = Array.isArray(params.files) ? params.files.map(String) : [];
    if (!files.length) throw new Error("upload requires files");
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      const checked = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: "function() { return this instanceof HTMLInputElement && this.type === 'file'; }",
        returnByValue: true,
      });
      this.assertRuntimeResult(checked);
      if (!checked.result?.value) throw new Error("upload selector must target an input[type=file]");
      await this.command(tab.id, "DOM.setFileInputFiles", { files, objectId });
      return { tab_id: tab.id, uploaded: true, selector: params.selector, files };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
  }

  async download(params) {
    const url = this.requireText(params.url, "download requires url");
    const options = {
      url,
      conflictAction: "uniquify",
      saveAs: Boolean(params.save_as),
    };
    if (params.file_name) options.filename = this.safeFileName(params.file_name, true);
    const downloadId = await chrome.downloads.download(options);
    return { download_id: downloadId, url, file_name: options.filename || "", started: true };
  }

  async downloads(params) {
    const cmd = String(params.cmd || "").toLowerCase();
    if (cmd === "list") {
      const query = {
        limit: Number.isInteger(params.max_results) ? params.max_results : 100,
        orderBy: ["-startTime"],
      };
      if (params.query) query.query = [String(params.query)];
      if (params.download_state) query.state = params.download_state;
      const items = await chrome.downloads.search(query);
      return { count: items.length, items: items.map((item) => this.downloadResult(item)) };
    }
    const downloadId = Number(params.download_id);
    if (cmd === "show") {
      const [item] = await chrome.downloads.search({ id: downloadId });
      if (!item) throw new Error("Unknown download: " + downloadId);
      await chrome.downloads.show(downloadId);
      return { shown: true, item: this.downloadResult(item) };
    }
    if (cmd === "wait") {
      return { item: await this.waitForDownload(downloadId, params.timeout_ms) };
    }
    throw new Error("downloads cmd must be list, wait, or show");
  }

  waitForDownload(downloadId, rawTimeout) {
    const timeoutMs = Number.isInteger(rawTimeout) ? rawTimeout : 60000;
    return new Promise((resolve, reject) => {
      let settled = false;
      const finish = async (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        chrome.downloads.onChanged.removeListener(listener);
        if (error) {
          reject(error);
          return;
        }
        try {
          const [item] = await chrome.downloads.search({ id: downloadId });
          if (!item) reject(new Error("Unknown download: " + downloadId));
          else resolve(this.downloadResult(item));
        } catch (searchError) {
          reject(searchError);
        }
      };
      const listener = (delta) => {
        if (delta.id !== downloadId) return;
        const state = delta.state?.current;
        if (state === "complete" || state === "interrupted") void finish();
      };
      const timeout = setTimeout(() => void finish(new Error("Download wait timed out")), timeoutMs);
      chrome.downloads.onChanged.addListener(listener);
      chrome.downloads.search({ id: downloadId }).then(([item]) => {
        if (!item) void finish(new Error("Unknown download: " + downloadId));
        else if (item.state === "complete" || item.state === "interrupted") void finish();
      }, (error) => void finish(error));
    });
  }

  downloadResult(item) {
    return {
      download_id: item.id,
      url: item.url || "",
      final_url: item.finalUrl || "",
      file_name: item.filename || "",
      mime_type: item.mime || "",
      state: item.state || "",
      paused: Boolean(item.paused),
      bytes_received: item.bytesReceived || 0,
      total_bytes: item.totalBytes || 0,
      start_time: item.startTime,
      end_time: item.endTime,
      error: item.error || "",
      exists: item.exists,
    };
  }

  async closeTab(params) {
    const tabId = this.parseTabID(params.tab_id);
    try {
      await chrome.tabs.remove(tabId);
      this.clearTab(tabId);
      return { closed: true, tab_id: tabId };
    } catch {
      this.clearTab(tabId);
      return { closed: false, tab_id: tabId, reason: "tab already closed" };
    }
  }

  async closeSession(params) {
    const existing = [];
    for (const tabId of this.sessionTabIDs(params)) {
      try {
        await chrome.tabs.get(tabId);
        existing.push(tabId);
      } catch {
        this.clearTab(tabId);
      }
    }
    if (existing.length) await chrome.tabs.remove(existing);
    existing.forEach((tabId) => this.clearTab(tabId));
    return { closed: existing.length, tab_ids: existing };
  }

  async finalizeRound(params) {
    const session = this.requireText(params.session, "finalize_round requires session");
    const roundID = this.requireText(params.round_id, "finalize_round requires round_id");
    const tabs = [...this.leaseByTab.entries()]
      .filter(([, lease]) => lease.session === session && lease.roundID === roundID)
      .sort(([left], [right]) => left - right);
    const result = { closed: 0, released: 0, handoff: 0 };
    for (const [tabId, lease] of tabs) {
      if (lease.mark === "handoff") {
        await this.hideCursor(tabId);
        lease.roundID = "";
        lease.mark = "";
        this.leaseByTab.set(tabId, lease);
        result.handoff += 1;
      } else if (lease.mark === "deliverable" || !lease.owned) {
        await this.releaseTab(tabId, lease.owned && lease.mark === "deliverable");
        result.released += 1;
      } else {
        await this.closeTab({ tab_id: tabId });
        result.closed += 1;
      }
    }
    return result;
  }

  async resolveElement(tabId, rawSelector) {
    const selector = this.requireText(rawSelector, "selector is required");
    const ref = selector.match(/^@?e\d+$/i) ? "@" + selector.replace(/^@/, "").toLowerCase() : "";
    if (ref) {
      const backendNodeId = this.refsByTab.get(tabId)?.get(ref);
      if (backendNodeId === undefined) throw new Error("Unknown ref " + selector + "; run snapshot again");
      const response = await this.command(tabId, "DOM.resolveNode", { backendNodeId });
      if (!response.object?.objectId) throw new Error("Unable to resolve ref " + selector);
      return response.object.objectId;
    }

    const response = await this.command(tabId, "Runtime.evaluate", {
      expression: "document.querySelector(" + JSON.stringify(selector) + ")",
      returnByValue: false,
    });
    this.assertRuntimeResult(response);
    if (!response.result?.objectId || response.result.subtype === "null") {
      throw new Error("No element matches selector: " + selector);
    }
    return response.result.objectId;
  }

  async releaseObject(tabId, objectId) {
    try {
      await this.command(tabId, "Runtime.releaseObject", { objectId });
    } catch {
      // 页面跳转会自动释放旧对象。
    }
  }

  assertRuntimeResult(response) {
    if (!response?.exceptionDetails) return;
    const message = response.exceptionDetails.exception?.description ||
      response.exceptionDetails.text ||
      "JavaScript execution failed";
    throw new Error(message);
  }

  async tabResult(tab) {
    let groupTitle = "";
    if (Number.isInteger(tab.groupId) && tab.groupId >= 0) {
      try {
        groupTitle = (await chrome.tabGroups.get(tab.groupId)).title || "";
      } catch {
        groupTitle = "";
      }
    }
    const lease = this.leaseByTab.get(tab.id);
    return {
      tab_id: tab.id,
      tab_ref: this.tabRef(tab.id),
      title: tab.title || "",
      url: tab.url || tab.pendingUrl || "",
      active: Boolean(tab.active),
      window_id: tab.windowId,
      group_id: tab.groupId,
      group_title: groupTitle,
      owned: Boolean(lease?.owned),
      round_id: lease?.roundID || "",
      mark: lease?.mark || "",
    };
  }

  claimTab(tabId, params, owned) {
    const session = String(params.session || "").trim();
    if (!session || !Number.isInteger(tabId) || tabId <= 0) return;
    const current = this.leaseByTab.get(tabId);
    const roundID = String(params.round_id || current?.roundID || "").trim();
    this.leaseByTab.set(tabId, {
      session,
      groupTitle: String(params.group_title || current?.groupTitle || "Nexus").trim() || "Nexus",
      owned: Boolean(owned || current?.owned),
      roundID,
      mark: current?.roundID === roundID ? current.mark || "" : "",
    });
  }

  touchLease(params) {
    if (!Number.isInteger(params.tab_id) || params.tab_id <= 0) return;
    const lease = this.leaseByTab.get(params.tab_id);
    const session = String(params.session || "").trim();
    const roundID = String(params.round_id || "").trim();
    if (!lease || !session || lease.session !== session || !roundID) return;
    if (lease.roundID !== roundID) lease.mark = "";
    lease.roundID = roundID;
    this.leaseByTab.set(params.tab_id, lease);
  }

  async inheritCreatedTab(details) {
    const sourceTabId = details?.sourceTabId;
    const tabId = details?.tabId;
    const lease = this.leaseByTab.get(sourceTabId);
    if (!lease || !Number.isInteger(tabId) || tabId <= 0) return;
    this.claimTab(tabId, {
      session: lease.session,
      group_title: lease.groupTitle,
      round_id: lease.roundID,
    }, true);
    try {
      await this.groupTab(tabId, lease.session, lease.groupTitle);
      const tab = await chrome.tabs.get(tabId);
      this.eventSink("tab_created", {
        session: lease.session,
        source_tab_ref: this.tabRef(sourceTabId),
        tab: await this.tabResult(tab),
        owned: true,
      });
    } catch {
      this.clearTab(tabId);
    }
  }

  handleTabRemoved(tabId) {
    const lease = this.leaseByTab.get(tabId);
    const tabRef = this.tabTokens.has(tabId) ? this.tabRef(tabId) : "";
    if (lease && tabRef) {
      this.eventSink("tab_removed", { session: lease.session, tab_ref: tabRef });
    }
    this.clearTab(tabId);
  }

  async handleTabUpdated(tabId, change, tab) {
    if (change.url || change.status === "loading") this.invalidateTabSnapshot(tabId);
    const lease = this.leaseByTab.get(tabId);
    if (!lease || !(change.url || change.title || change.status)) return;
    try {
      const current = tab?.id === tabId ? tab : await chrome.tabs.get(tabId);
      this.eventSink("tab_updated", {
        session: lease.session,
        tab: await this.tabResult(current),
        status: change.status || current.status || "",
      });
    } catch {
      this.handleTabRemoved(tabId);
    }
  }

  async handleTabActivated(tabId) {
    const lease = this.leaseByTab.get(tabId);
    if (!lease) return;
    try {
      this.eventSink("tab_activated", {
        session: lease.session,
        tab: await this.tabResult(await chrome.tabs.get(tabId)),
      });
    } catch {
      this.handleTabRemoved(tabId);
    }
  }

  async groupTab(tabId, rawSession, rawTitle) {
    const session = String(rawSession || "").trim();
    if (!session) return;
    const title = String(rawTitle || "Nexus").trim() || "Nexus";
    let groupId = this.groupBySession.get(session);
    try {
      if (groupId === undefined) {
        groupId = await chrome.tabs.group({ tabIds: [tabId] });
      } else {
        await chrome.tabs.group({ tabIds: [tabId], groupId });
      }
    } catch {
      groupId = await chrome.tabs.group({ tabIds: [tabId] });
    }
    this.groupBySession.set(session, groupId);
    const hash = [...session].reduce((sum, character) => sum + character.codePointAt(0), 0);
    await chrome.tabGroups.update(groupId, {
      title,
      color: GROUP_COLORS[hash % GROUP_COLORS.length],
      collapsed: false,
    });
  }

  matchesURL(rawURL, rawPattern) {
    const url = String(rawURL || "");
    const pattern = String(rawPattern || "").trim();
    if (!url || !pattern) return false;
    if (pattern.includes("*")) {
      const source = pattern
        .split("*")
        .map((part) => part.replace(/[.*+?^{}$()|[\]\\]/g, "\\$&"))
        .join(".*");
      return new RegExp(source, "i").test(url);
    }
    if (url.toLowerCase().includes(pattern.toLowerCase())) return true;
    try {
      const expected = new URL(pattern).hostname.toLowerCase();
      const actual = new URL(url).hostname.toLowerCase();
      return actual === expected || actual.endsWith("." + expected);
    } catch {
      try {
        const actual = new URL(url).hostname.toLowerCase();
        const expected = pattern.toLowerCase().replace(/^\./, "");
        return actual === expected || actual.endsWith("." + expected);
      } catch {
        return false;
      }
    }
  }

  async attachDebugger(tabId) {
    if (this.attachedTabs.has(tabId)) return;
    try {
      await chrome.debugger.detach({ tabId });
    } catch {
      // 扩展重启后可能不知道上一条调试会话，先清理再连接。
    }
    try {
      await chrome.debugger.attach({ tabId }, "1.3");
      this.attachedTabs.add(tabId);
      await chrome.debugger.sendCommand({ tabId }, "Page.enable");
      await chrome.debugger.sendCommand({ tabId }, "Runtime.enable");
      await chrome.debugger.sendCommand({ tabId }, "Log.enable");
      this.consoleTabs.add(tabId);
      if (!this.consoleEntries.has(tabId)) this.consoleEntries.set(tabId, []);
    } catch (error) {
      this.attachedTabs.delete(tabId);
      try {
        await chrome.debugger.detach({ tabId });
      } catch {
        // attach 失败时不一定存在可清理的调试会话。
      }
      throw new Error("Cannot attach debugger to tab " + tabId + ": " + error.message);
    }
  }

  async releaseTab(tabId, removeFromGroup = false) {
    await this.hideCursor(tabId);
    if (this.attachedTabs.has(tabId)) {
      try {
        await chrome.debugger.detach({ tabId });
      } catch {
        // 页面关闭或调试会话先行结束时，只需清理本地租约。
      }
    }
    if (removeFromGroup) {
      try {
        await chrome.tabs.ungroup(tabId);
      } catch {
        // 标签页已离组或关闭时仍可直接交还用户。
      }
    }
    this.clearTab(tabId);
  }

  async command(tabId, method, params = {}) {
    await this.attachDebugger(tabId);
    return chrome.debugger.sendCommand({ tabId }, method, params);
  }

  async getTab(rawTabId) {
    return chrome.tabs.get(this.parseTabID(rawTabId));
  }

  parseTabID(value) {
    if (!Number.isInteger(value) || value <= 0) throw new Error("A valid tab_id is required");
    return value;
  }

  tabRef(tabId) {
    tabId = this.parseTabID(tabId);
    if (!this.browserInstanceID || !this.browserGeneration) {
      throw new Error("Browser identity is not initialized");
    }
    let token = this.tabTokens.get(tabId);
    if (!token) {
      token = crypto.randomUUID();
      this.tabTokens.set(tabId, token);
    }
    return JSON.stringify([this.browserInstanceID, this.browserGeneration, tabId, token]);
  }

  parseTabRef(value) {
    let parts;
    try {
      parts = JSON.parse(this.requireText(value, "A valid tab_ref is required"));
    } catch {
      throw new Error("A valid tab_ref is required; run list_tabs again");
    }
    if (!Array.isArray(parts) || parts.length !== 4 ||
        parts[0] !== this.browserInstanceID || parts[1] !== this.browserGeneration ||
        !Number.isInteger(parts[2]) || parts[2] <= 0 ||
        this.tabTokens.get(parts[2]) !== parts[3]) {
      throw new Error("Stale tab_ref; run list_tabs or attach_active again");
    }
    return parts[2];
  }

  tabRefs(values) {
    if (!Array.isArray(values)) return [];
    const tabIds = [];
    for (const value of values) {
      try {
        tabIds.push(this.parseTabRef(value));
      } catch {
        // 已关闭或来自旧扩展代次的标签页不再属于当前 Session。
      }
    }
    return [...new Set(tabIds)];
  }

  sessionTabIDs(params) {
    const tabIds = new Set(this.tabRefs(params.tab_refs));
    const session = String(params.session || "").trim();
    if (session) {
      for (const [tabId, lease] of this.leaseByTab) {
        if (lease.session === session) tabIds.add(tabId);
      }
    }
    return [...tabIds];
  }

  requireText(value, message) {
    const result = String(value ?? "").trim();
    if (!result) throw new Error(message);
    return result;
  }

  cleanText(value) {
    return String(value ?? "").replace(/\s+/g, " ").trim();
  }

  compactSnapshotText(value) {
    const text = this.cleanText(value);
    if (text.length <= SNAPSHOT_TEXT_MAX_CHARS) return text;
    return text.slice(0, SNAPSHOT_TEXT_MAX_CHARS - 1) + "…";
  }

  safeFileName(value, preserveExtension = false) {
    const cleaned = String(value || "page").replace(/[<>:"/\\|?*\u0000-\u001f]/g, "_").trim();
    const result = cleaned || "page";
    return preserveExtension ? result : result.replace(/\.pdf$/i, "");
  }

  waitForLoad(tabId) {
    return new Promise((resolve, reject) => {
      let settled = false;
      const finish = (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        chrome.tabs.onUpdated.removeListener(listener);
        if (error) reject(error); else resolve();
      };
      const timeout = setTimeout(() => {
        finish(new Error("Page load timed out after 30 seconds"));
      }, 30000);
      const listener = (updatedTabId, change) => {
        if (updatedTabId !== tabId || change.status !== "complete") return;
        finish();
      };
      chrome.tabs.onUpdated.addListener(listener);
      chrome.tabs.get(tabId).then((current) => {
        if (current.status === "complete") finish();
      }, finish);
    });
  }

  waitForNextLoad(tabId) {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        chrome.tabs.onUpdated.removeListener(listener);
        reject(new Error("Page load timed out after 30 seconds"));
      }, 30000);
      const listener = (updatedTabId, change) => {
        if (updatedTabId !== tabId || change.status !== "complete") return;
        clearTimeout(timeout);
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      };
      chrome.tabs.onUpdated.addListener(listener);
    });
  }

  invalidateTabSnapshot(tabId) {
    this.refsByTab.delete(tabId);
    this.refByBackendByTab.delete(tabId);
    this.nextRefByTab.delete(tabId);
    this.snapshotByTab.delete(tabId);
  }

  clearTab(tabId) {
    this.attachedTabs.delete(tabId);
    this.invalidateTabSnapshot(tabId);
    this.networkTabs.delete(tabId);
    this.networkRequests.delete(tabId);
    this.consoleTabs.delete(tabId);
    this.consoleEntries.delete(tabId);
    this.dialogs.delete(tabId);
    this.leaseByTab.delete(tabId);
    this.tabTokens.delete(tabId);
  }
}

class BrowserClient {
  constructor(controller) {
    this.controller = controller;
    this.socket = null;
    this.pendingSocket = null;
    this.currentURL = "";
    this.connecting = false;
    this.generation = 0;
    this.queue = Promise.resolve();
    this.controller.setEventSink((event, data) => {
      if (this.socket) this.send(this.socket, { type: "browser.event", event, data });
    });
  }

  async start() {
    const stored = await chrome.storage.local.get(STORAGE_INSTANCE_ID);
    let instanceID = String(stored[STORAGE_INSTANCE_ID] || "").trim();
    if (!instanceID) {
      instanceID = crypto.randomUUID();
      await chrome.storage.local.set({ [STORAGE_INSTANCE_ID]: instanceID });
    }
    this.controller.setIdentity(instanceID, BROWSER_GENERATION);
    chrome.alarms.create(RECONNECT_ALARM, { periodInMinutes: 0.5 });
    chrome.alarms.onAlarm.addListener((alarm) => {
      if (alarm.name === RECONNECT_ALARM) void this.reconcile();
    });
    chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
      if (message?.target === "nexus-browser-offscreen") return false;
      this.handleControlMessage(message).then(sendResponse, (error) => {
        sendResponse({ ok: false, error: error?.message || String(error) });
      });
      return true;
    });
    await this.setBadge("…", "#64748b");
    await this.reconcile();
  }

  async handleControlMessage(message) {
    switch (message?.type) {
      case "GET_STATUS":
        return { ok: true, ...await this.status() };
      case "CONNECT":
        await this.configure(message.url, true);
        return { ok: true, ...await this.status() };
      case "DISCONNECT":
        await this.configure(undefined, false);
        return { ok: true, ...await this.status() };
      case "TEST_CONNECTION":
        await this.testConnection(message.url || DEFAULT_ENDPOINTS[0]);
        return { ok: true };
      case "RESET_URL":
        await chrome.storage.local.remove(STORAGE_URL);
        await this.configure(undefined, true);
        return { ok: true, ...await this.status() };
      case "OPEN_IN_NEXUS":
        await this.openInNexus();
        return { ok: true };
      default:
        throw new Error("Unknown extension command");
    }
  }

  async configure(rawURL, enabled) {
    if (rawURL !== undefined) {
      const url = String(rawURL || "").trim();
      if (url) {
        await chrome.storage.local.set({ [STORAGE_URL]: this.normalizeEndpoint(url) });
      } else {
        await chrome.storage.local.remove(STORAGE_URL);
      }
    }
    await chrome.storage.local.set({ [STORAGE_ENABLED]: enabled });
    this.generation += 1;
    this.closeSocket();
    if (enabled) {
      await this.reconcile();
    } else {
      await this.setBadge("OFF", "#94a3b8");
    }
  }

  async status() {
    const config = await chrome.storage.local.get([STORAGE_URL, STORAGE_ENABLED]);
    const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
    const activeURL = String(activeTab?.url || "");
    return {
      connected: this.socket?.readyState === WebSocket.OPEN,
      enabled: config[STORAGE_ENABLED] !== false,
      extension_version: chrome.runtime.getManifest().version,
      configured_url: config[STORAGE_URL] || "",
      current_url: this.currentURL,
      default_url: DEFAULT_ENDPOINTS[0],
      active_tab: activeTab?.id === undefined ? null : {
        controlled: this.controller.leaseByTab.has(activeTab.id),
        controllable: isControllableURL(activeURL),
        title: activeTab.title || "",
        url: activeURL,
      },
    };
  }

  async openInNexus(info = {}, tab) {
    const activeTab = tab ?? (await chrome.tabs.query({ active: true, currentWindow: true }))[0];
    const prompt = buildNexusContextPrompt(info, activeTab);
    if (!prompt || ![info?.linkUrl, info?.srcUrl, info?.pageUrl, activeTab?.url].some(isControllableURL)) {
      throw new Error("当前页面不支持交给 Nexus");
    }
    await chrome.tabs.create({ url: buildNexusLaunchURL(prompt) });
  }

  async reconcile() {
    if (this.connecting || this.socket?.readyState === WebSocket.OPEN) return;
    const generation = this.generation;
    const config = await chrome.storage.local.get([STORAGE_URL, STORAGE_ENABLED]);
    if (config[STORAGE_ENABLED] === false) {
      await this.setBadge("OFF", "#94a3b8");
      return;
    }
    this.connecting = true;
    try {
      const endpoints = config[STORAGE_URL]
        ? [this.normalizeEndpoint(config[STORAGE_URL])]
        : DEFAULT_ENDPOINTS;
      for (const endpoint of endpoints) {
        if (generation !== this.generation) return;
        if (await this.tryEndpoint(endpoint, generation)) return;
      }
      this.currentURL = "";
      await this.setBadge("—", "#94a3b8");
    } finally {
      this.connecting = false;
      if (generation !== this.generation) setTimeout(() => void this.reconcile(), 0);
    }
  }

  tryEndpoint(endpoint, generation) {
    return new Promise((resolve) => {
      const socket = new WebSocket(endpoint, SUBPROTOCOL);
      this.pendingSocket = socket;
      let accepted = false;
      let settled = false;
      const finish = (success) => {
        if (settled) return;
        settled = true;
        clearTimeout(timeout);
        if (this.pendingSocket === socket) this.pendingSocket = null;
        resolve(success);
      };
      const timeout = setTimeout(() => {
        socket.close();
        finish(false);
      }, 4000);

      socket.addEventListener("open", () => {
        socket.send(JSON.stringify({
          type: "browser.ready",
          protocol_version: PROTOCOL_VERSION,
          extension_version: chrome.runtime.getManifest().version,
          browser_name: browserNameFromUserAgent(globalThis.navigator?.userAgent),
          browser_instance_id: this.controller.browserInstanceID,
          browser_generation: this.controller.browserGeneration,
        }));
      });
      socket.addEventListener("message", (event) => {
        let message;
        try {
          message = JSON.parse(event.data);
        } catch {
          return;
        }
        if (!accepted && message.type === "browser.accepted") {
          if (generation !== this.generation) {
            socket.close();
            finish(false);
            return;
          }
          accepted = true;
          this.socket = socket;
          this.currentURL = endpoint;
          void this.setBadge("ON", "#16a34a");
          finish(true);
          return;
        }
        if (accepted) this.handleMessage(socket, message);
      });
      socket.addEventListener("close", () => {
        if (this.socket === socket) {
          this.socket = null;
          this.currentURL = "";
          void this.setBadge("—", "#94a3b8");
          setTimeout(() => void this.reconcile(), 1000);
        }
        if (!accepted) finish(false);
      });
      socket.addEventListener("error", () => {
        if (!accepted) {
          socket.close();
          finish(false);
        }
      });
    });
  }

  handleMessage(socket, message) {
    if (message.type === "browser.ping") {
      this.send(socket, { type: "browser.pong" });
      return;
    }
    if (message.type !== "browser.command" || !message.id || !message.action) return;
    this.queue = this.queue.then(async () => {
      try {
        const result = await this.controller.execute(message.action, message.params || {});
        this.send(socket, { type: "browser.result", id: message.id, result });
      } catch (error) {
        this.send(socket, {
          type: "browser.result",
          id: message.id,
          error: error?.message || String(error),
        });
      }
    });
  }

  testConnection(rawURL) {
    const endpoint = this.normalizeEndpoint(rawURL);
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(endpoint, SUBPROTOCOL);
      const timeout = setTimeout(() => {
        socket.close();
        reject(new Error("Connection test timed out"));
      }, 4000);
      socket.addEventListener("open", () => {
        clearTimeout(timeout);
        socket.close();
        resolve();
      });
      socket.addEventListener("error", () => {
        clearTimeout(timeout);
        socket.close();
        reject(new Error("Unable to connect"));
      });
    });
  }

  normalizeEndpoint(rawURL) {
    const parsed = new URL(String(rawURL || "").trim());
    if (parsed.protocol !== "ws:" && parsed.protocol !== "wss:") {
      throw new Error("URL must use ws:// or wss://");
    }
    return parsed.href;
  }

  closeSocket() {
    const socket = this.socket;
    const pendingSocket = this.pendingSocket;
    this.socket = null;
    this.pendingSocket = null;
    this.currentURL = "";
    if (socket) socket.close();
    if (pendingSocket && pendingSocket !== socket) pendingSocket.close();
  }

  send(socket, payload) {
    if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify(payload));
  }

  async setBadge(text, color) {
    await chrome.action.setBadgeBackgroundColor({ color });
    await chrome.action.setBadgeText({ text });
  }
}

const browserClient = new BrowserClient(new BrowserController());
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.remove(CONTEXT_MENU_ID, () => {
    void chrome.runtime.lastError;
    chrome.contextMenus.create({
      contexts: ["page", "selection", "link", "image"],
      documentUrlPatterns: ["http://*/*", "https://*/*"],
      id: CONTEXT_MENU_ID,
      title: "在 Nexus 中询问",
    });
  });
});
chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId === CONTEXT_MENU_ID) {
    void browserClient.openInNexus(info, tab).catch((error) => {
      console.warn("Unable to open Nexus context", error);
    });
  }
});
void browserClient.start();
