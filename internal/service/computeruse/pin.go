// INPUT: the nexus-cua SDK version pinned in go.mod.
// OUTPUT: the only runtime release and archive digests this host accepts.
// POS: Computer Use supply-chain pin; bump together with the SDK in go.mod.
package computeruse

// pinnedCUAVersion must equal the github.com/nexus-research-lab/nexus-cua/sdk/go
// version in go.mod. TestPinnedCUAVersionMatchesGoModule enforces this.
const pinnedCUAVersion = "0.2.0"

// pinnedCUAArchiveSHA256 binds each installable release asset to a digest
// reviewed in source. The release's own .sha256 file only proves transport
// integrity; this map proves the asset is the one this build was tested with.
// Copy values from the release's SHA256SUMS when bumping pinnedCUAVersion.
var pinnedCUAArchiveSHA256 = map[string]string{
	"nexuscua-0.2.0-macos-arm64.tar.gz": "1fe8d14dfad62e52626a2fb619de3d6ca48e19777b0b4c31fa8e2f5eb5459982",
	"nexuscua-0.2.0-windows-x86_64.zip": "75fd7f9d4bf5fc566da10490923071ab512063b7109e0dcbb87494c89b93875a",
}

func pinnedCUAReleaseAPIURL() string {
	return "https://api.github.com/repos/nexus-research-lab/nexus-cua/releases/tags/v" + pinnedCUAVersion
}
