# settings/

L3 | 父级: web/src/lib

## 职责

- `preferences-normalization.ts` 统一 Config 与 Settings 共同依赖的用户偏好值清洗和 Agent Options 合并规则。

本目录只保存跨层复用的纯规则，不读取运行时快照、不发请求，也不反向依赖 Feature。
