// INPUT: 直接凭证与 OAuth Connector 详情，以及用户填写和提交动作。
// OUTPUT: 证明授权弹窗随步骤标题命名、复用共享排版/表单并只提交完整的业务凭证。
// POS: Connector 授权弹窗 DOM 合同；Device Flow 时序由相邻 poller 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { LOCALE_STORAGE_KEY, MESSAGES, type Locale } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ConnectorDetail } from "@/types/capability/connector";

import { ConnectorCredentialDialog } from "./connector-credential-dialog";
import { ConnectorOAuthClientDialog } from "./connector-oauth-client-dialog";
import { FeishuAppConnectionDialog } from "./feishu/feishu-app-connection-dialog";

const AMAP_DETAIL = {
  auth_type: "api_key",
  category: "travel",
  connection_state: "disconnected",
  connector_id: "amap",
  description: "查询地点和路线。",
  docs_url: "https://lbs.amap.com/",
  features: [],
  icon: "/icon/connector/amap.svg",
  is_configured: false,
  kind: "connector",
  name: "amap",
  scopes: [],
  status: "available",
  title: "高德地图",
} satisfies ConnectorDetail;

const FEISHU_DETAIL = {
  ...AMAP_DETAIL,
  auth_type: "oauth2",
  category: "productivity",
  connector_id: "feishu-docx",
  description: "访问飞书云文档。",
  name: "feishu-docx",
  oauth_client_configured: false,
  title: "飞书云文档",
} satisfies ConnectorDetail;

beforeEach(() => localStorage.setItem(LOCALE_STORAGE_KEY, "zh"));

describe("Connector authorization dialogs", () => {
  it("keeps busy Feishu connection choices named and inert until the operation settles", async () => {
    const user = userEvent.setup();
    const onScan = vi.fn();
    const props = { isOpen: true, onClose: vi.fn(), onConnectManually: vi.fn(), onScan };
    const view = (busy: boolean) => <I18nProvider><FeishuAppConnectionDialog {...props} busy={busy} /></I18nProvider>;
    const { rerender } = render(view(false));
    const dialog = screen.getByRole("dialog", { name: "连接飞书云文档" });
    await user.click(screen.getByRole("button", { name: /扫码连接/ }));
    expect(onScan).toHaveBeenCalledOnce();

    rerender(view(true));
    for (const name of [/扫码连接/, /手动配置/]) {
      const choice = screen.getByRole("button", { name });
      expect(choice.getAttribute("aria-disabled")).toBe("true");
      expect(choice.hasAttribute("tabindex")).toBe(false);
      await user.click(choice);
      await user.keyboard("{Enter} ");
    }
    expect(onScan).toHaveBeenCalledOnce();
    expect(screen.queryByRole("textbox", { name: /App ID/ })).toBeNull();
    rerender(view(false));
    await user.click(screen.getByRole("button", { name: /手动配置/ }));
    expect(screen.getByRole("dialog", { name: "手动连接飞书" })).toBe(dialog);
    expect(screen.getByRole("textbox", { name: /App ID/ })).toBeTruthy();
  });

  it("keeps direct credential copy semantic and submits the exact trimmed value", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <I18nProvider>
        <ConnectorCredentialDialog
          busy={false}
          detail={AMAP_DETAIL}
          onClose={vi.fn()}
          onSave={onSave}
        />
      </I18nProvider>,
    );

    expect(screen.getByRole("dialog", { name: "连接 高德地图" })).toBeTruthy();
    expect(screen.getByText("粘贴高德开放平台的 Web 服务 Key。").className)
      .toContain("ui-type-supporting");
    await user.type(screen.getByLabelText("API Key*"), "  secret-key  ");
    await user.click(screen.getByRole("button", { name: "连接" }));

    expect(onSave).toHaveBeenCalledWith("amap", "secret-key");
  });

  it("renders OAuth callback data as shared code typography and submits complete fields", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ConnectorOAuthClientDialog
          busy={false}
          detail={FEISHU_DETAIL}
          onClose={vi.fn()}
          onDelete={vi.fn()}
          onSave={onSave}
        />
      </I18nProvider>,
    );

    expect(screen.getByRole("dialog", { name: "配置 飞书云文档" })).toBeTruthy();
    expect(screen.getByText(/先在飞书开放平台应用添加回调地址/).className)
      .toContain("ui-type-supporting");
    expect(container.querySelector("code")?.className).toContain("ui-type-code");

    await user.type(screen.getByLabelText("Client ID*"), "client-id");
    await user.type(screen.getByLabelText("Client Secret*"), "client-secret");
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(onSave).toHaveBeenCalledWith("feishu-docx", "client-id", "client-secret");
  });
});


it("translates a direct credential dialog without clearing its draft or submitting on locale change", async () => {
  const user = userEvent.setup();
  const onSave = vi.fn();
  const onClose = vi.fn();
  const view = (locale: Locale) => {
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce(
      (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key],
    );
    return <I18N_CONTEXT.Provider value={{locale, t, setLocale: vi.fn()}}>
      <ConnectorCredentialDialog busy={false} detail={AMAP_DETAIL} onSave={onSave} onClose={onClose} />
    </I18N_CONTEXT.Provider>;
  };
  const {rerender} = render(view("zh"));
  const input = screen.getByLabelText("API Key*") as HTMLInputElement;
  await user.type(input, "  user-key  ");
  rerender(view("en"));
  expect(screen.getByRole("dialog", {name: "Connect 高德地图"})).toBeTruthy();
  expect(screen.getByText("Paste the Web Service Key from Amap Open Platform.")).toBeTruthy();
  expect(screen.getByRole("link", {name: "View documentation"}).getAttribute("href")).toBe(AMAP_DETAIL.docs_url);
  expect(screen.getByLabelText("API Key*")).toBe(input);
  expect(input.value).toBe("  user-key  ");
  expect(input.type).toBe("password");
  expect(onSave).not.toHaveBeenCalled();
  expect(onClose).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", {name: "Connect"}));
  expect(onSave).toHaveBeenCalledExactlyOnceWith("amap", "user-key");
});

it("keeps OAuth credentials when changing language and resets them for a different configuration", async () => {
  const user = userEvent.setup();
  const onSave = vi.fn();
  const onDelete = vi.fn();
  const original = {...FEISHU_DETAIL, oauth_client_id: "existing-client", oauth_client_configured: true};
  const view = (locale: Locale, detail = original) => {
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce(
      (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key],
    );
    return <I18N_CONTEXT.Provider value={{locale, t, setLocale: vi.fn()}}>
      <ConnectorOAuthClientDialog busy={false} detail={detail} onSave={onSave} onDelete={onDelete} onClose={vi.fn()} />
    </I18N_CONTEXT.Provider>;
  };
  const {rerender} = render(view("zh"));
  const secret = screen.getByLabelText("Client Secret*") as HTMLInputElement;
  await user.type(secret, "entered-secret");
  rerender(view("en"));
  expect(screen.getByRole("dialog", {name: "Configure 飞书云文档"})).toBeTruthy();
  expect(screen.getByText("Add the callback URL to your Feishu Open Platform app, then enter the app credentials.")).toBeTruthy();
  expect(screen.getByRole("button", {name: "Copy callback URL"})).toBeTruthy();
  expect(screen.getByLabelText("Client Secret*")).toBe(secret);
  expect(secret.value).toBe("entered-secret");
  expect(secret.placeholder).toBe("Enter the secret again to save");
  expect(onSave).not.toHaveBeenCalled();
  expect(onDelete).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", {name: "Save"}));
  expect(onSave).toHaveBeenCalledExactlyOnceWith("feishu-docx", "existing-client", "entered-secret");
  rerender(view("en", {...original, oauth_client_id: "replacement-client"}));
  expect((screen.getByLabelText("Client ID*") as HTMLInputElement).value).toBe("replacement-client");
  expect((screen.getByLabelText("Client Secret*") as HTMLInputElement).value).toBe("");
});

it.each(["credential", "oauth"] as const)("associates %s labels with their own dialog instance", async (kind) => {
  const user = userEvent.setup();
  const dialog = () => kind === "credential"
    ? <ConnectorCredentialDialog detail={AMAP_DETAIL} busy={false} onClose={vi.fn()} onSave={vi.fn()} />
    : <ConnectorOAuthClientDialog detail={FEISHU_DETAIL} busy={false} onClose={vi.fn()} onDelete={vi.fn()} onSave={vi.fn()} />;
  render(<I18nProvider>{dialog()}{dialog()}</I18nProvider>);
  const forms = Array.from(document.querySelectorAll("form"));
  expect(forms).toHaveLength(2);
  const ids = forms.flatMap((form) => Array.from(form.querySelectorAll("input"), (input) => input.id));
  expect(new Set(ids).size).toBe(ids.length);
  const topForm = forms[1];
  const labels = topForm.querySelectorAll<HTMLLabelElement>("label[for]");
  expect(labels.length).toBe(kind === "credential" ? 1 : 2);
  for (const label of labels) {
    await user.click(label);
    const input = label.control as HTMLInputElement;
    expect(input.closest("form")).toBe(topForm);
    expect(document.activeElement).toBe(input);
    await user.keyboard("draft");
    expect(input.value).toBe("draft");
  }
  forms[0].querySelectorAll("input").forEach((input) => expect(input.value).toBe(""));
});

it.each(["credential", "oauth"] as const)("rejects %s form submissions while busy and preserves the draft", async (kind) => {
  const user = userEvent.setup();
  const onSave = vi.fn();
  const onClose = vi.fn();
  const view = (busy: boolean) => <I18nProvider>{kind === "credential"
    ? <ConnectorCredentialDialog detail={AMAP_DETAIL} busy={busy} onClose={onClose} onSave={onSave} />
    : <ConnectorOAuthClientDialog detail={FEISHU_DETAIL} busy={busy} onClose={onClose} onDelete={vi.fn()} onSave={onSave} />
  }</I18nProvider>;
  const { rerender } = render(view(false));
  if (kind === "oauth") await user.type(screen.getByLabelText("Client ID*"), "client-id");
  const input = screen.getByLabelText(kind === "credential" ? "API Key*" : "Client Secret*");
  await user.type(input, "secret");
  const form = input.closest("form")!;
  rerender(view(true));
  expect(form.getAttribute("aria-busy")).toBe("true");
  fireEvent.submit(form);
  expect(onSave).not.toHaveBeenCalled();
  rerender(view(false));
  expect(form.getAttribute("aria-busy")).toBe("false");
  expect((input as HTMLInputElement).value).toBe("secret");
  fireEvent.submit(form);
  expect(onSave).toHaveBeenCalledExactlyOnceWith(...(kind === "credential"
    ? ["amap", "secret"] : ["feishu-docx", "client-id", "secret"]));
});

it("preserves the Feishu manual step and credentials across language changes", async () => {
  const user = userEvent.setup();
  const save = vi.fn();
  const view = (locale: Locale) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <FeishuAppConnectionDialog isOpen busy={false} onClose={vi.fn()} onScan={vi.fn()} onConnectManually={save} />
  </I18N_CONTEXT.Provider>;
  const { rerender } = render(view("zh"));
  await user.click(screen.getByRole("button", { name: /手动配置/ }));
  const id = screen.getByLabelText("App ID*");
  const secret = screen.getByLabelText("App Secret*");
  await user.type(id, " app-id ");
  await user.type(secret, " app-secret ");
  rerender(view("en"));
  expect(screen.getByRole("dialog", { name: "Connect Feishu manually" })).toBeTruthy();
  expect(screen.getByLabelText("App ID*")).toBe(id);
  expect(screen.getByLabelText("App Secret*")).toBe(secret);
  expect((secret as HTMLInputElement).value).toBe(" app-secret ");
  expect(save).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Continue" }));
  expect(save).toHaveBeenCalledExactlyOnceWith("app-id", "app-secret");
  await user.click(screen.getByRole("button", { name: "Back" }));
  expect(screen.getByRole("button", { name: /Connect with QR code/ })).toBeTruthy();
});
