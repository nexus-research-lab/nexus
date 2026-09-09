# 国际化入口

- `messages.ts` 只暴露 Locale、翻译键和完整语言目录，不保存具体文案。
- `i18n-provider.tsx` 只负责语言选择、持久化和参数格式化。
- 具体文案按职责归入 `catalog/`，禁止恢复单文件巨型语言对象。

- `locale-settings.ts` 是无 Provider/目录依赖的语言偏好入口；存储读写失败不阻断渲染或当前页面语言切换，启动失败面也复用它。
