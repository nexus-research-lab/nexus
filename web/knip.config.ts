import type { KnipConfig } from "knip";

const config: KnipConfig = {
  entry: [
    "src/entries/ui-gallery.tsx",
    // Playwright 按路径字符串注入的 ui-gallery 夹具。
    "src/dev/ui-gallery/mount-*-fixture.ts",
    "browser-tests/**/*.{ts,mjs}",
  ],
  // 同文件内部消费的导出不算死代码；只报告全仓无人引用的成员。
  ignoreExportsUsedInFile: true,
  ignore: [
    // TS↔Go 协议契约层：src/types/** 下的类型镜像 Go 后端 domain/protocol 结构体
    // （如 internal/runtime/permission、internal/protocol codegen、scheduled-task/connector 等 capability API），
    // 作为前端对外露出的 typed surface 刻意保留；当前 FE 未直接消费，不计为死代码。
    "src/types/**/*.ts",
    // Humation MIT 上游快照保持原样，不按本仓导出规则修改。
    "src/shared/lib/humation/assets.ts",
  ],
  ignoreDependencies: [
    // eslint flat config 工具链基础包，未被直接 import，作为显式版本约束保留
    "@eslint/js",
  ],
};

export default config;
