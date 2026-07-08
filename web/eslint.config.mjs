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
    files: ["src/**/*.{ts,tsx}", "vite.config.ts"],
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
      "@typescript-eslint": tseslint.plugin,
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      "jsx-a11y": jsxA11y,
    },
    rules: {
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
