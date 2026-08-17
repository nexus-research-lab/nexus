// INPUT: 宿主签发给当前 Agent runtime 的 Nexus command broker 地址与稳定 capability。
// OUTPUT: nxs/Claude、CLI 与本机 broker 共用且不可由模型覆盖的环境变量和请求头名称。
// POS: 面向 Agent 的 Nexus CLI 跨进程最小 wire；具体领域命令由各领域协议定义。
package protocol

const (
	NexusCommandPathEnvName            = "NEXUS_COMMAND_PATH"
	NexusCommandBrokerURLEnvName       = "NEXUS_COMMAND_BROKER_URL"
	NexusCommandCapabilityTokenEnvName = "NEXUS_COMMAND_CAPABILITY_TOKEN"
	NexusCommandCapabilityHeader       = "X-Nexus-Runtime-Command-Capability"
)
