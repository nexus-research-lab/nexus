# Sidebar 空状态原语

本目录只保留可被不同业务侧栏复用的空状态。

- 展示原语不得读取应用路由、Sidebar Store、业务 API 或 Feature Tour。
- 业务导航项归所属 Feature；没有第二个真实消费者时不得在这里预留重命名、删除或外观变体。
- 应用宽侧栏归 `features/navigation/sidebar/`，引导定义归 `features/onboarding/`。
