chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message?.target !== "nexus-browser-offscreen") return false;
  const operation = message.type === "CLIPBOARD_WRITE"
    ? navigator.clipboard.writeText(String(message.text ?? "")).then(() => ({ ok: true }))
    : navigator.clipboard.readText().then((text) => ({ ok: true, text }));
  operation.then(sendResponse, (error) => {
    sendResponse({ ok: false, error: error?.message || String(error) });
  });
  return true;
});
