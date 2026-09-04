// INPUT: Composer 权限请求、密钥输入、主决策与范围菜单用户事件。
// OUTPUT: 证明共享 Form/SplitButton 仍提交精确 permission payload 与禁用状态。
// POS: Composer 权限确认面行为测试；权限建议投影细节由纯模型测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ComposerPermissionSurface } from "./composer-permission-surface";

function renderPermissionSurface(
  props: Partial<React.ComponentProps<typeof ComposerPermissionSurface>> = {},
) {
  const onResponse = vi.fn(() => true);
  render(
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
        {...props}
      />
    </I18N_CONTEXT.Provider>,
  );
  return onResponse;
}

describe("ComposerPermissionSurface", () => {
  it("keeps deny and allow-once as independent shared actions", async () => {
    const user = userEvent.setup();
    const onResponse = renderPermissionSurface();

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
    const onResponse = renderPermissionSurface({
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
    const onResponse = renderPermissionSurface({
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
});
