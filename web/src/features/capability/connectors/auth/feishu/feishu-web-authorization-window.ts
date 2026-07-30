const FEISHU_AUTH_WINDOW_NAME = "nexus-feishu-docx-auth";
const FEISHU_AUTH_WINDOW_FEATURES =
  "popup=yes,width=520,height=700,resizable=yes,scrollbars=yes";

type FeishuAuthWindowOpener = () => Window | null;
type FeishuAuthNavigationScheduler = (navigate: () => void) => void;
type FeishuAuthLoadingRenderer = (popup: Window) => void;

function openFeishuAuthWindow(): Window | null {
  return window.open(
    "",
    FEISHU_AUTH_WINDOW_NAME,
    FEISHU_AUTH_WINDOW_FEATURES,
  );
}

function scheduleFeishuAuthNavigation(navigate: () => void): void {
  window.setTimeout(navigate, 50);
}

function renderFeishuAuthLoadingPage(popup: Window): void {
  const document = popup.document;
  document.documentElement.lang = "zh-CN";
  document.title = "Nexus · 飞书授权";
  document.body.replaceChildren();
  document.body.style.cssText = [
    "margin:0",
    "min-height:100vh",
    "display:grid",
    "place-items:center",
    "background:#f7f7f5",
    "color:#202124",
    "font-family:-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif",
  ].join(";");

  const panel = document.createElement("main");
  panel.style.cssText = [
    "width:min(360px,calc(100vw - 48px))",
    "box-sizing:border-box",
    "padding:28px",
    "border:1px solid #e4e4e1",
    "border-radius:16px",
    "background:#fff",
    "box-shadow:0 16px 44px rgba(0,0,0,.08)",
    "text-align:center",
  ].join(";");

  const indicator = document.createElement("div");
  indicator.textContent = "N";
  indicator.style.cssText = [
    "display:grid",
    "place-items:center",
    "width:44px",
    "height:44px",
    "margin:0 auto 16px",
    "border:1px solid #dededb",
    "border-radius:50%",
    "font-size:20px",
    "font-weight:650",
  ].join(";");

  const title = document.createElement("h1");
  title.textContent = "正在打开飞书授权页";
  title.style.cssText = "margin:0 0 8px;font-size:18px";

  const description = document.createElement("p");
  description.textContent = "应用信息已确认，正在载入当前用户授权页面。";
  description.style.cssText = "margin:0;color:#666;line-height:1.6;font-size:14px";

  panel.append(indicator, title, description);
  document.body.append(panel);
}

/**
 * Web 只在飞书返回 App ID、进入用户授权阶段后尝试打开授权窗口。
 * 异步弹窗可能被浏览器拦截，调用方必须保留明确的用户点击兜底。
 */
export class FeishuWebAuthorizationWindow {
  private popup: Window | null = null;
  private currentUrl = "";
  private navigationVersion = 0;

  constructor(
    private readonly opener: FeishuAuthWindowOpener = openFeishuAuthWindow,
    private readonly navigationScheduler: FeishuAuthNavigationScheduler =
      scheduleFeishuAuthNavigation,
    private readonly loadingRenderer: FeishuAuthLoadingRenderer =
      renderFeishuAuthLoadingPage,
  ) {}

  open(url: string): boolean {
    const normalizedUrl = url.trim();
    if (!normalizedUrl) {
      return false;
    }
    let popup = this.activePopup();
    if (!popup) {
      popup = this.opener();
      if (!popup) {
        return false;
      }
      this.popup = popup;
      this.currentUrl = "";
      try {
        popup.opener = null;
        this.loadingRenderer(popup);
      } catch {
        // 即时加载提示失败不应阻止用户继续授权。
      }
    }
    if (this.currentUrl !== normalizedUrl) {
      this.currentUrl = normalizedUrl;
      const navigationVersion = ++this.navigationVersion;
      this.navigationScheduler(() => {
        if (
          navigationVersion !== this.navigationVersion
          || popup.closed
          || this.popup !== popup
        ) {
          return;
        }
        popup.location.href = normalizedUrl;
      });
    }
    popup.focus();
    return true;
  }

  close(): void {
    const popup = this.activePopup();
    if (popup) {
      popup.close();
    }
    this.popup = null;
    this.currentUrl = "";
    this.navigationVersion += 1;
  }

  private activePopup(): Window | null {
    if (!this.popup || this.popup.closed) {
      this.popup = null;
      this.currentUrl = "";
      this.navigationVersion += 1;
      return null;
    }
    return this.popup;
  }
}
