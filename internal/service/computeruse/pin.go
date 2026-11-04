// INPUT: the nexus-cua SDK version pinned in go.mod.
// OUTPUT: the only runtime release and archive digests this host accepts.
// POS: Computer Use supply-chain pin; bump together with the SDK in go.mod.
package computeruse

// pinnedCUAVersion must equal the github.com/nexus-research-lab/nexus-cua/sdk/go
// version in go.mod. TestPinnedCUAVersionMatchesGoModule enforces this.
const pinnedCUAVersion = "0.2.0-rc.5"

// pinnedCUAArchiveSHA256 binds each installable release asset to a digest
// reviewed in source. The release's own .sha256 file only proves transport
// integrity; this map proves the asset is the one this build was tested with.
// Copy values from the release's SHA256SUMS when bumping pinnedCUAVersion.
var pinnedCUAArchiveSHA256 = map[string]string{
	"nexuscua-0.2.0-rc.5-macos-arm64.tar.gz": "23f0adad8e4d20119dd4e318fd6f1e2184d365c264cad255f1dcfa2636a719ee",
	"nexuscua-0.2.0-rc.5-windows-x86_64.zip": "bbdcfa1d1a9e0fd5fc912cbd677727dc9bbdf4874548696e7baac27915b61681",
}

func pinnedCUAReleaseAPIURL() string {
	return "https://api.github.com/repos/nexus-research-lab/nexus-cua/releases/tags/v" + pinnedCUAVersion
}
