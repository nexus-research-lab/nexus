# shared/navigation/

L2 | 父级: web/src/shared

`route-paths.ts` 是 App、Page 和 Feature 共用的唯一无运行态路由合同，拥有路径模板、资源身份编码以及内部 Conversation/外部 Session 的 canonical 路由投影。

它不读取 Store、登录态或浏览器位置，不执行导航，不挂载页面。路由匹配与 Provider 仍归 App，当前资源选择与导航动作仍归消费方；禁止恢复 App 路径转发壳或在业务页复制编码规则。
