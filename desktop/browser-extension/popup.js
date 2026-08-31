// INPUT: User-initiated extension commands and connection-status reads from the background worker.
// OUTPUT: Popup state plus safe, operation-specific feedback with impact and a concrete next action.
// POS: Browser-extension user boundary; raw background errors are diagnostic-only and never rendered.

const statusNode = document.querySelector("#status");
const hintNode = document.querySelector("#hint");
const messageNode = document.querySelector("#message");
const urlNode = document.querySelector("#url");
const versionNode = document.querySelector("#version");
const connectButton = document.querySelector("#connect");
const activeTabNode = document.querySelector("#active-tab");
const activeTabTitleNode = document.querySelector("#active-tab-title");
const activeTabURLNode = document.querySelector("#active-tab-url");
const activeTabStateNode = document.querySelector("#active-tab-state");
const askNexusButton = document.querySelector("#ask-nexus");
const buttons = [...document.querySelectorAll("button")];

const failureCopy = Object.freeze({
  connectionSetting: "连接设置更改没有完成，是否已经生效暂时无法确认。当前网页和 Nexus 中已有的会话、任务、文件不会因此改变。重新打开扩展核对地址与连接状态，再决定是否重试。",
  connectionTest: "连接测试没有完成。测试不会修改连接设置、当前网页或 Nexus 中已有的内容。确认地址和 Nexus Desktop 的运行状态后再试。",
  openInNexus: "无法确认 Nexus 页面是否已经打开。当前网页内容没有被修改。先检查标签页；如果没有新的 Nexus 页面，再重试。",
  statusRead: "扩展无法读取当前连接状态。连接设置、当前网页和 Nexus 中已有的内容不会因这次读取失败而改变。确认 Nexus Desktop 已运行，然后重新打开扩展。",
});

async function send(type, payload = {}) {
  const response = await chrome.runtime.sendMessage({ type, ...payload });
  if (!response?.ok) throw new Error(response?.error || "操作失败");
  return response;
}

function render(status) {
  urlNode.placeholder = status.default_url || "ws://127.0.0.1:34343/nexus/v1/internal/browser/ws";
  if (!urlNode.value) urlNode.value = status.configured_url || "";
  versionNode.textContent = status.extension_version || chrome.runtime.getManifest().version;
  renderActiveTab(status.active_tab);
  if (status.connected) {
    statusNode.textContent = "已连接到 Nexus";
    statusNode.dataset.state = "connected";
    hintNode.textContent = "现在可以直接在 Nexus 中使用 Browser，无需保持弹窗打开。";
    connectButton.hidden = true;
  } else if (!status.enabled) {
    statusNode.textContent = "已手动断开";
    statusNode.dataset.state = "disconnected";
    hintNode.textContent = "自动连接已暂停，需要时可随时重新开启。";
    connectButton.textContent = "连接 Nexus";
    connectButton.hidden = false;
  } else {
    statusNode.textContent = "等待 Nexus";
    statusNode.dataset.state = "disconnected";
    hintNode.textContent = "确认 Nexus Desktop 已运行，扩展会自动连接。";
    connectButton.textContent = "立即重试";
    connectButton.hidden = false;
  }
}

function renderActiveTab(tab) {
  activeTabNode.hidden = !tab;
  askNexusButton.hidden = !tab?.controllable;
  if (!tab) return;
  activeTabTitleNode.textContent = tab.title || "未命名页面";
  try {
    activeTabURLNode.textContent = new URL(tab.url).host;
  } catch {
    activeTabURLNode.textContent = tab.url;
  }
  activeTabStateNode.textContent = tab.controlled
    ? "Nexus 正在使用此页面"
    : tab.controllable
      ? "可在 Nexus 任务中使用"
      : "此浏览器页面不支持控制";
}

async function run(task, successMessage, failureMessage) {
  buttons.forEach((button) => { button.disabled = true; });
  messageNode.dataset.state = "progress";
  messageNode.textContent = "正在处理…";
  try {
    const result = await task();
    if (result?.connected !== undefined) render(result);
    messageNode.dataset.state = "success";
    messageNode.textContent = successMessage;
  } catch (error) {
    console.warn("[Nexus Browser] popup command failed", error);
    messageNode.dataset.state = "error";
    messageNode.textContent = failureMessage;
  } finally {
    buttons.forEach((button) => { button.disabled = false; });
  }
}

document.querySelector("#connect").addEventListener("click", () => {
  void run(
    () => send("CONNECT", { url: urlNode.value.trim() }),
    "连接设置已保存",
    failureCopy.connectionSetting,
  );
});
askNexusButton.addEventListener("click", () => {
  void run(
    () => send("OPEN_IN_NEXUS"),
    "已交给 Nexus",
    failureCopy.openInNexus,
  );
});
document.querySelector("#disconnect").addEventListener("click", () => {
  void run(
    () => send("DISCONNECT"),
    "已断开",
    failureCopy.connectionSetting,
  );
});
document.querySelector("#test").addEventListener("click", () => {
  void run(
    () => send("TEST_CONNECTION", { url: urlNode.value.trim() || urlNode.placeholder }),
    "地址可连接",
    failureCopy.connectionTest,
  );
});
document.querySelector("#reset").addEventListener("click", () => {
  urlNode.value = "";
  void run(
    () => send("RESET_URL"),
    "已恢复默认地址",
    failureCopy.connectionSetting,
  );
});

async function refresh() {
  try {
    render(await send("GET_STATUS"));
  } catch (error) {
    console.warn("[Nexus Browser] status read failed", error);
    statusNode.textContent = "无法读取连接状态";
    statusNode.dataset.state = "disconnected";
    hintNode.textContent = failureCopy.statusRead;
  }
}

void refresh();
setInterval(() => void refresh(), 1500);
