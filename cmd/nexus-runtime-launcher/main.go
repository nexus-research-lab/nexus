// nexus-runtime-launcher 是 root-owned 的最小 runtime 身份与 Landlock 边界。
package main

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeidentity"
)

func main() {
	os.Exit(runtimeidentity.Run(os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}
