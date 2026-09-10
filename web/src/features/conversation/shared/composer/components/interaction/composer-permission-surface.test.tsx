// INPUT: Composer 权限请求、密钥输入、主决策与范围菜单用户事件。
// OUTPUT: 证明请求/禁用切换关闭范围菜单，失败保留密钥，重试只提交 runtime 原始建议。
// POS: Composer 权限确认面行为测试；权限建议投影细节由纯模型测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { ComposerPermissionSurface } from "./composer-permission-surface";

function renderPermissionSurface(
  props: Partial<React.ComponentProps<typeof ComposerPermissionSurface>> = {},
) {
  const onResponse = vi.fn(() => true);
  const node = (next: typeof props) => (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ComposerPermissionSurface
        interactionDisabled={false}
        kind="permission"
        onResponse={onResponse}
        permission={{
          request_id: "permission-1",
          tool_input: {},
          tool_name: "WebSearch",
        }}
        total={1}
        {...next}
      />
    </I18N_CONTEXT.Provider>
  );
  const view = render(node(props));
  return { onResponse, rerender: (next: typeof props) => view.rerender(node(next)) };
}

describe("ComposerPermissionSurface", () => {
  it("keeps deny and allow-once as independent shared actions", async () => {
    const user = userEvent.setup();
    const { onResponse } = renderPermissionSurface();

    expect(screen.getByRole("group", {
      name: "composer.permission_allow_once",
    }).getAttribute("data-slot")).toBe("split-button");

    await user.click(screen.getByRole("button", {
      name: "composer.permission_allow_once",
    }));
    await user.click(screen.getByRole("button", {
      name: "composer.permission_deny",
    }));

    expect(onResponse).toHaveBeenNthCalledWith(1, {
      decision: "allow",
      request_id: "permission-1",
    });
    expect(onResponse).toHaveBeenNthCalledWith(2, {
      decision: "deny",
      request_id: "permission-1",
    });
  });

  it("uses masked shared inputs and enables allow only after every secret is filled", async () => {
    const user = userEvent.setup();
    const { onResponse } = renderPermissionSurface({
      permission: {
        configuration_secret_slots: [
          { id: "token", path: "service.token" },
          { id: "password", path: "service.password" },
        ],
        request_id: "permission-secrets",
        tool_input: {},
        tool_name: "ConfigureService",
      },
    });
    const allow = screen.getByRole("button", {
      name: "composer.permission_allow_once",
    }) as HTMLButtonElement;
    const token = screen.getByLabelText("service.token") as HTMLInputElement;
    const password = screen.getByLabelText("service.password") as HTMLInputElement;

    expect(token.type).toBe("password");
    expect(token.className).toContain("input-shell");
    expect(allow.disabled).toBe(true);
    await user.type(token, "token-value");
    expect(allow.disabled).toBe(true);
    await user.type(password, "password-value");
    expect(allow.disabled).toBe(false);
    await user.click(allow);

    expect(onResponse).toHaveBeenCalledWith({
      configuration_secrets: {
        password: "password-value",
        token: "token-value",
      },
      decision: "allow",
      request_id: "permission-secrets",
    });
  });

  it("opens the adjacent scope menu without replacing the allow-once command", async () => {
    const user = userEvent.setup();
    const { onResponse } = renderPermissionSurface({
      permission: {
        automation: {
          allow_task: true,
          job_id: "task-1",
          kind: "tool",
          policy_revision: 4,
          task_name: "Daily report",
        },
        request_id: "permission-task",
        source: "automation",
        tool_input: { query: "latest research" },
        tool_name: "WebSearch",
      },
    });

    await user.click(screen.getByRole("button", {
      name: "composer.permission_choose_scope",
    }));
    const taskScope = await screen.findByRole("menuitem", {
      name: /composer\.permission_allow_task/,
    });
    await user.click(taskScope);

    expect(onResponse).toHaveBeenCalledWith({
      automation_scope: "task",
      decision: "allow",
      request_id: "permission-task",
    });
  });

  it("closes a scope menu on request changes and submits only the newly opened request", async () => {
    const user = userEvent.setup();
    const permission: PendingPermission = {
      request_id: "first", tool_name: "Bash", tool_input: {},
      suggestions: [{ type: "addRules", behavior: "allow", destination: "session", rules: [{ tool_name: "Bash", rule_content: "git status" }] }],
    };
    const { onResponse, rerender } = renderPermissionSurface({ permission });
    const open = () => user.click(screen.getByRole("button", { name: "composer.permission_choose_scope" }));
    await open();
    expect(screen.getByRole("menu")).toBeTruthy();
    const next = { ...permission, request_id: "second" };
    rerender({ permission: next });
    expect(screen.queryByRole("menu")).toBeNull();
    expect(onResponse).not.toHaveBeenCalled();
    await open();
    await user.click(screen.getAllByRole("menuitem")[1]);
    expect(onResponse).toHaveBeenCalledWith({
      request_id: "second", decision: "allow", updated_permissions: permission.suggestions,
    });
  });

  it.each(["disabled", "secret", "choices"])("closes a scope menu when %s invalidates its actions, without reviving it later", async (reason) => {
    const user = userEvent.setup();
    const permission: PendingPermission = {
      request_id: "scope-request", tool_name: "Bash", tool_input: {},
      configuration_secret_slots: [{ id: "key", path: "service.key" }],
      suggestions: [{ type: "addRules", behavior: "allow", destination: "session", rules: [{ tool_name: "Bash" }] }],
    };
    const { onResponse, rerender } = renderPermissionSurface({ permission });
    await user.type(screen.getByLabelText("service.key"), "secret-value");
    await user.click(screen.getByRole("button", { name: "composer.permission_choose_scope" }));
    const oldAction = screen.getAllByRole("menuitem")[1];
    if (reason === "secret") fireEvent.change(screen.getByLabelText("service.key"), { target: { value: "" } });
    else rerender({ permission: reason === "choices" ? { ...permission, suggestions: [] } : permission, interactionDisabled: reason === "disabled" });
    expect(screen.queryByRole("menu")).toBeNull();
    fireEvent.click(oldAction);
    expect(onResponse).not.toHaveBeenCalled();
    rerender({ permission });
    if (reason === "secret") await user.type(screen.getByLabelText("service.key"), "secret-value");
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("retains secrets after rejected delivery and forwards the unchanged runtime suggestion on retry", async () => {
    const user = userEvent.setup();
    const permission: PendingPermission = {
      request_id: "retry-request", tool_name: "Bash", tool_input: {},
      configuration_secret_slots: [{ id: "key", path: "service.key" }],
      suggestions: [{ type: "addRules", behavior: "allow", destination: "localSettings", rules: [{ tool_name: "Bash", rule_content: "git diff:*" }] }],
    };
    const { onResponse } = renderPermissionSurface({ permission });
    onResponse.mockReturnValueOnce(false);
    const secret = screen.getByLabelText("service.key") as HTMLInputElement;
    await user.type(secret, "secret-value");
    const chooseRule = async () => {
      await user.click(screen.getByRole("button", { name: "composer.permission_choose_scope" }));
      await user.click(screen.getAllByRole("menuitem")[1]);
    };
    await chooseRule();
    expect(secret.value).toBe("secret-value");
    await chooseRule();
    expect(onResponse).toHaveBeenCalledTimes(2);
    expect(onResponse.mock.calls[0]).toEqual(onResponse.mock.calls[1]);
    expect(onResponse).toHaveBeenLastCalledWith({
      configuration_secrets: { key: "secret-value" }, decision: "allow", request_id: "retry-request",
      updated_permissions: permission.suggestions,
    });
    expect(secret.value).toBe("");
    expect(screen.queryByRole("menu")).toBeNull();
  });

});
