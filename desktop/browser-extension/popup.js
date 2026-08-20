const statusNode = document.querySelector("#status");
const messageNode = document.querySelector("#message");
const urlNode = document.querySelector("#url");
const buttons = [...document.querySelectorAll("button")];

async function send(type, payload = {}) {
  const response = await chrome.runtime.sendMessage({ type, ...payload });
  if (!response?.ok) throw new Error(response?.error || "操作失败");
  return response;
}

function render(status) {
  urlNode.placeholder = status.default_url || "ws://127.0.0.1:34343/nexus/v1/internal/webbridge/ws";
  if (!urlNode.value) urlNode.value = status.configured_url || "";
  if (status.connected) {
    statusNode.textContent = "已连接";
    statusNode.dataset.state = "connected";
  } else if (!status.enabled) {
    statusNode.textContent = "已手动断开";
    statusNode.dataset.state = "disconnected";
  } else {
    statusNode.textContent = "等待 Nexus";
    statusNode.dataset.state = "disconnected";
  }
}

async function run(task, successMessage) {
  buttons.forEach((button) => { button.disabled = true; });
  messageNode.textContent = "正在处理…";
  try {
    const result = await task();
    if (result?.connected !== undefined) render(result);
    messageNode.textContent = successMessage;
  } catch (error) {
    messageNode.textContent = error?.message || String(error);
  } finally {
    buttons.forEach((button) => { button.disabled = false; });
  }
}

document.querySelector("#connect").addEventListener("click", () => {
  void run(() => send("CONNECT", { url: urlNode.value.trim() }), "连接设置已保存");
});
document.querySelector("#disconnect").addEventListener("click", () => {
  void run(() => send("DISCONNECT"), "已断开");
});
document.querySelector("#test").addEventListener("click", () => {
  void run(() => send("TEST_CONNECTION", { url: urlNode.value.trim() || urlNode.placeholder }), "地址可连接");
});
document.querySelector("#reset").addEventListener("click", () => {
  urlNode.value = "";
  void run(() => send("RESET_URL"), "已恢复默认地址");
});

void send("GET_STATUS").then(render, (error) => {
  statusNode.textContent = error?.message || "无法读取状态";
  statusNode.dataset.state = "disconnected";
});
