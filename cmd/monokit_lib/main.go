// Command monokit_lib is a smoke binary for the library.
//
// monokit_lib is a library — nothing ships this binary and nothing should
// depend on its output. It exists so that `just build-all` has something to
// link, which is what actually verifies the library cross-compiles: in
// particular the shutdown_notifier_linux.go / shutdown_notifier_stub.go build
// tag split, which no test can cover because a test only ever builds for the
// platform it runs on.
//
// Running it prints what the library resolved from the environment, which is
// occasionally useful when a plugin's config is not being picked up.
package main

import (
	"fmt"
	"os"

	lib "github.com/monobilisim/monokit_lib"
)

// set by -ldflags "-X 'main.version=...'" in the justfile
var version string

func main() {
	if version == "" {
		version = "devel"
	}

	fmt.Printf("monokit_lib %s\n", version)

	if err := lib.InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("project identifier: %s\n", lib.GlobalConfig.ProjectIdentifier)
	fmt.Printf("hostname:           %s\n", lib.GlobalConfig.Hostname)
	fmt.Printf("log dir:            %s\n", lib.LogDir)
	fmt.Printf("db dir:             %s\n", lib.DbDir)
	fmt.Printf("plugins dir:        %s\n", lib.PluginsDir)
}
