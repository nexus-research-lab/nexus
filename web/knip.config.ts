import type { KnipConfig } from "knip";

const config: KnipConfig = {
  entry: [
    "src/entries/app.tsx",
    "src/entries/settings.tsx",
    "src/entries/oauth-callback.tsx",
  ],
  ignore: [
    // TS↔Go 协议契约层：src/types/** 下的类型镜像 Go 后端 domain/protocol 结构体
    // （如 internal/runtime/permission、internal/protocol codegen、scheduled-task/connector 等 capability API），
    // 作为前端对外露出的 typed surface 刻意保留；当前 FE 未直接消费，不计为死代码。
    "src/types/**/*.ts",
  ],
  ignoreDependencies: [
    // eslint flat config 工具链基础包，未被直接 import，作为显式版本约束保留
    "@eslint/js",
  ],
};

export default config;
