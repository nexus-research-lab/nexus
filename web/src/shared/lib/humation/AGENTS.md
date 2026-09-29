# 内置 Humation

- 上游 https://github.com/humation-labs/humation，固定提交 f47bb3dee04f21e377b6114141737280b8daa68b，素材版本 1.0.1。代码与素材保留 MIT LICENSE，不使用 npm 包。
- create-avatar.ts、types.ts、assets.ts 为上游源码快照；仅增加文件契约头，assets.ts 另调整类型导入，保留原始 SVG 与英文上游注释以便核对。
- avatar.ts 持有 Nexus 的 h1: 版本化头像标识、边界校验和图片投影。字段顺序与部件 ID 不得重排/复用；新增素材只追加，保存显式部件避免扩容改变已有头像。
- nexus-parts.ts 为 Nexus 自有配件，坐标沿用 Humation item 层；不改动上游素材文件。
- 构建分发的许可副本位于 `web/public/licenses/humation.txt`，更新上游许可时同步两处。
- `create-avatar.ts` 额外导出上游 `fnv1a`，Nexus 复用同一散列为背景选择固定八色之一；背景写入 h1 标识，读取已保存头像不重新随机。
