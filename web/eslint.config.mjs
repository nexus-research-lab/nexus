import { frontendRules } from "./scripts/frontend-lint-rules.mjs";
import globals from "globals";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import jsxA11y from "eslint-plugin-jsx-a11y";

export default tseslint.config(
  {
    ignores: [
      "dist/**",
      "build/**",
      "coverage/**",
      "node_modules/**",
    ],
  },
  {
    files: ["src/**/*.{ts,tsx,mjs}", "visual-review/**/*.{ts,tsx}", "browser-tests/**/*.ts", "smoke-tests/**/*.ts", "vite.config.ts", "vitest.config.ts", "playwright.config.ts", "playwright.smoke.config.ts"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: "latest",
        sourceType: "module",
        ecmaFeatures: {
          jsx: true,
        },
      },
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    plugins: {
      nexus: { rules: frontendRules },
      "@typescript-eslint": tseslint.plugin,
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      "jsx-a11y": jsxA11y,
    },
    rules: {
      "nexus/dependency-direction": "error",
      "nexus/file-contract": "error",
      "nexus/semantic-style-owners": "error",
      ...reactHooks.configs.recommended.rules,
      ...jsxA11y.flatConfigs.recommended.rules,
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXAttribute[name.name=/^[a-z][A-Za-z0-9]*_[A-Za-z0-9_]*$/]",
          message: "前端 JSX props 使用 camelCase；协议对象字段不要放在 JSX attribute 上。",
        },
        {
          selector: "ExportNamedDeclaration > FunctionDeclaration[id.name=/^[a-z][A-Za-z0-9]*_[A-Za-z0-9_]*$/]",
          message: "前端 exported 函数名使用 camelCase。",
        },
        {
          selector: "ExportNamedDeclaration > VariableDeclaration > VariableDeclarator[id.name=/^[a-z][A-Za-z0-9]*_[A-Za-z0-9_]*$/]",
          message: "前端 exported 变量名使用 camelCase。",
        },
      ],
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
    },
  },
);
