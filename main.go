package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/xrzks/fpn/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version string

func main() {
	if err := cli.New(buildVersion()).Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// buildVersion prefers the version stamped by -ldflags, then the module or VCS
// version the Go toolchain records in the binary.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
