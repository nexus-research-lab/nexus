const PROTOCOL_VERSION = "1";
const SUBPROTOCOL = "nexus.webbridge.v1";
const DEFAULT_ENDPOINTS = [
  "ws://127.0.0.1:34343/nexus/v1/internal/webbridge/ws",
  "ws://127.0.0.1:8010/nexus/v1/internal/webbridge/ws",
];
const RECONNECT_ALARM = "nexus-webbridge-reconnect";
const STORAGE_URL = "webbridge_url";
const STORAGE_ENABLED = "webbridge_enabled";
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
const GROUP_COLORS = ["blue", "purple", "cyan", "green", "yellow", "orange", "red", "pink", "grey"];
const PAPER_FORMATS = {
  letter: [8.5, 11],
  legal: [8.5, 14],
  a4: [8.27, 11.69],
  a3: [11.69, 16.54],
  tabloid: [11, 17],
};

class BrowserController {
  constructor() {
    this.attachedTabs = new Set();
    this.refsByTab = new Map();
    this.networkTabs = new Set();
    this.networkRequests = new Map();
    this.groupBySession = new Map();
    this.platform = null;

    chrome.tabs.onRemoved.addListener((tabId) => this.clearTab(tabId));
    chrome.debugger.onDetach.addListener((source) => {
      if (source.tabId !== undefined) {
        this.attachedTabs.delete(source.tabId);
        this.networkTabs.delete(source.tabId);
      }
    });
    chrome.debugger.onEvent.addListener((source, method, params) => {
      if (source.tabId !== undefined) {
        this.handleNetworkEvent(source.tabId, method, params);
      }
    });
    chrome.tabGroups.onRemoved.addListener((group) => {
      for (const [session, groupId] of this.groupBySession) {
        if (groupId === group.id) this.groupBySession.delete(session);
      }
    });
  }

  async execute(action, params = {}) {
    switch (action) {
      case "list_tabs":
        return this.listTabs(params);
      case "attach_active":
        return this.attachActive();
      case "attach_tab":
        return this.attachTab(params);
      case "navigate":
        return this.navigate(params);
      case "find_tab":
        return this.findTab(params);
      case "evaluate":
        return this.evaluate(params);
      case "network":
        return this.network(params);
      case "snapshot":
        return this.snapshot(params);
      case "click":
        return this.click(params);
      case "fill":
        return this.fill(params);
      case "mouse_click":
        return this.mouseClick(params);
      case "cdp":
        return this.rawCDP(params);
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
      case "close_tab":
      case "close":
        return this.closeTab(params);
      case "close_session":
        return this.closeSession(params);
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
    return { ...await this.tabResult(tab), owned: created, created };
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
      for (const tabId of this.tabIDs(params.tab_ids)) {
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
    return { ...await this.tabResult(tab), borrowed, owned: false };
  }

  async listTabs(params) {
    const tabs = [];
    for (const tabId of this.tabIDs(params.tab_ids)) {
      try {
        tabs.push(await this.tabResult(await chrome.tabs.get(tabId)));
      } catch {
        // 标签页可能在宿主发出请求前已由用户关闭。
      }
    }
    return { tabs };
  }

  async attachActive() {
    const [tab] = await chrome.tabs.query({ active: true, lastFocusedWindow: true });
    if (!tab?.id) throw new Error("No active browser tab found");
    await this.attachDebugger(tab.id);
    return { ...await this.tabResult(tab), borrowed: true, owned: false };
  }

  async attachTab(params) {
    const tab = await this.getTab(params.tab_id);
    await this.attachDebugger(tab.id);
    return { ...await this.tabResult(tab), borrowed: true, owned: false };
  }

  async evaluate(params) {
    const tab = await this.getTab(params.tab_id);
    const response = await this.command(tab.id, "Runtime.evaluate", {
      expression: this.requireText(params.code, "evaluate requires code"),
      returnByValue: true,
      awaitPromise: false,
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

  async snapshot(params) {
    const tab = await this.getTab(params.tab_id);
    const response = await this.command(tab.id, "Accessibility.getFullAXTree");
    const rendered = this.buildAccessibilityTree(tab.id, response.nodes || []);
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || "",
      tree: rendered.tree,
      refs: rendered.refCount,
    };
  }

  buildAccessibilityTree(tabId, nodes) {
    const byId = new Map(nodes.map((node) => [node.nodeId, node]));
    const childIds = new Set(nodes.flatMap((node) => node.childIds || []));
    const roots = nodes.filter((node) => !childIds.has(node.nodeId));
    const refs = new Map();
    const visited = new Set();
    let nextRef = 1;

    const build = (nodeId) => {
      const node = byId.get(nodeId);
      if (!node || visited.has(nodeId)) return [];
      visited.add(nodeId);
      const children = (node.childIds || []).flatMap(build);
      const role = this.cleanText(node.role?.value).toLowerCase();
      const name = this.cleanText(node.name?.value);
      const value = this.cleanText(node.value?.value);
      if (node.ignored || ((!role || role === "none" || role === "generic") && !name && !value)) {
        return children;
      }
      const item = { role: role || "unknown" };
      if (name) item.name = name;
      if (value && value !== name) item.value = value;
      const description = this.cleanText(node.description?.value);
      if (description) item.description = description;
      for (const key of ["disabled", "focused", "checked", "selected", "expanded", "level"]) {
        const property = node.properties?.find((candidate) => candidate.name === key)?.value?.value;
        if (property !== undefined) item[key] = property;
      }
      if (INTERACTIVE_ROLES.has(role) && node.backendDOMNodeId !== undefined) {
        const ref = "@e" + nextRef++;
        refs.set(ref, node.backendDOMNodeId);
        item.ref = ref;
      }
      if (children.length) item.children = children;
      return [item];
    };

    const tree = (roots.length ? roots : nodes.slice(0, 1)).flatMap((node) => build(node.nodeId));
    this.refsByTab.set(tabId, refs);
    return { tree, refCount: refs.size };
  }

  async click(params) {
    const tab = await this.getTab(params.tab_id);
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      const response = await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: "function() { this.scrollIntoView({ block: 'center', inline: 'center' }); this.click(); return { tag: this.tagName }; }",
        returnByValue: true,
        userGesture: true,
      });
      this.assertRuntimeResult(response);
      return { tab_id: tab.id, clicked: true, selector: params.selector, ...(response.result?.value || {}) };
    } finally {
      await this.releaseObject(tab.id, objectId);
    }
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

  async mouseClick(params) {
    const tab = await this.getTab(params.tab_id);
    const objectId = await this.resolveElement(tab.id, params.selector);
    try {
      await this.command(tab.id, "Runtime.callFunctionOn", {
        objectId,
        functionDeclaration: "function() { this.scrollIntoView({ block: 'center', inline: 'center' }); }",
      });
      const box = await this.command(tab.id, "DOM.getBoxModel", { objectId });
      const quad = box.model?.border || box.model?.content;
      if (!Array.isArray(quad) || quad.length < 8) throw new Error("Element has no clickable layout box");
      const x = (quad[0] + quad[2] + quad[4] + quad[6]) / 4;
      const y = (quad[1] + quad[3] + quad[5] + quad[7]) / 4;
      await this.command(tab.id, "Input.dispatchMouseEvent", { type: "mouseMoved", x, y, button: "none" });
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mousePressed", x, y, button: "left", buttons: 1, clickCount: 1,
      });
      await this.command(tab.id, "Input.dispatchMouseEvent", {
        type: "mouseReleased", x, y, button: "left", buttons: 0, clickCount: 1,
      });
      return { tab_id: tab.id, clicked: true, selector: params.selector, x, y };
    } finally {
      await this.releaseObject(tab.id, objectId);
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
      captureBeyondViewport: Boolean(params.selector),
    };
    if (format === "jpeg" && Number.isInteger(params.quality)) commandParams.quality = params.quality;
    if (params.selector) commandParams.clip = await this.elementClip(tab.id, params.selector);
    const response = await this.command(tab.id, "Page.captureScreenshot", commandParams);
    if (!response.data) throw new Error("Chrome returned an empty screenshot");
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || "",
      mime_type: "image/" + format,
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
    for (const tabId of this.tabIDs(params.tab_ids)) {
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
    return {
      tab_id: tab.id,
      title: tab.title || "",
      url: tab.url || tab.pendingUrl || "",
      active: Boolean(tab.active),
      window_id: tab.windowId,
      group_id: tab.groupId,
      group_title: groupTitle,
    };
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
    } catch (error) {
      throw new Error("Cannot attach debugger to tab " + tabId + ": " + error.message);
    }
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

  tabIDs(values) {
    return Array.isArray(values) ? [...new Set(values.filter((value) => Number.isInteger(value) && value > 0))] : [];
  }

  requireText(value, message) {
    const result = String(value ?? "").trim();
    if (!result) throw new Error(message);
    return result;
  }

  cleanText(value) {
    return String(value ?? "").replace(/\s+/g, " ").trim();
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

  clearTab(tabId) {
    this.attachedTabs.delete(tabId);
    this.refsByTab.delete(tabId);
    this.networkTabs.delete(tabId);
    this.networkRequests.delete(tabId);
  }
}

class BridgeClient {
  constructor(controller) {
    this.controller = controller;
    this.socket = null;
    this.pendingSocket = null;
    this.currentURL = "";
    this.connecting = false;
    this.generation = 0;
    this.queue = Promise.resolve();
  }

  async start() {
    chrome.alarms.create(RECONNECT_ALARM, { periodInMinutes: 0.5 });
    chrome.alarms.onAlarm.addListener((alarm) => {
      if (alarm.name === RECONNECT_ALARM) void this.reconcile();
    });
    chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
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
    return {
      connected: this.socket?.readyState === WebSocket.OPEN,
      enabled: config[STORAGE_ENABLED] !== false,
      configured_url: config[STORAGE_URL] || "",
      current_url: this.currentURL,
      default_url: DEFAULT_ENDPOINTS[0],
    };
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
          type: "bridge.ready",
          protocol_version: PROTOCOL_VERSION,
          extension_version: chrome.runtime.getManifest().version,
        }));
      });
      socket.addEventListener("message", (event) => {
        let message;
        try {
          message = JSON.parse(event.data);
        } catch {
          return;
        }
        if (!accepted && message.type === "bridge.accepted") {
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
    if (message.type === "bridge.ping") {
      this.send(socket, { type: "bridge.pong" });
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

const bridge = new BridgeClient(new BrowserController());
void bridge.start();
