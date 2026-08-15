// INPUT: 宿主签发给当前 Agent runtime 的 nexuscfg broker 地址与短期能力凭据。
// OUTPUT: CLI、runtime 环境和 HTTP handler 共用且不可由模型参数覆盖的变量名。
// POS: nexuscfg 跨进程调用宿主配置服务的最小 wire 常量。
package protocol

const (
	NexusConfigBrokerURLEnvName       = "NEXUSCFG_BROKER_URL"
	NexusConfigCapabilityTokenEnvName = "NEXUSCFG_CAPABILITY_TOKEN"
	NexusConfigCapabilityHeader       = "X-Nexus-Configuration-Capability"
)
